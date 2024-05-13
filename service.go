package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/actors/builtin/verifreg"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/types"
	cbor "github.com/ipfs/go-ipld-cbor"
	"gorm.io/gorm"
)

type Service struct {
	api          API
	adtStore     adt.Store
	db           *gorm.DB
	shutdownChan chan struct{}
}

func NewService(ctx context.Context, db *gorm.DB, api API, shutdownChan chan struct{}) *Service {
	tbs := blockstore.NewTieredBstore(blockstore.NewAPIBlockstore(api), blockstore.NewMemory())
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(tbs))
	s := &Service{
		db:           db,
		api:          api,
		adtStore:     adtStore,
		shutdownChan: shutdownChan,
	}
	go s.run()
	return s
}

func (s *Service) run() {
	tk := time.NewTicker(5 * time.Second)
	defer tk.Stop()
	for {
		select {
		case <-s.shutdownChan:
			log.Infof("shutting down service")
			return
		case <-tk.C:
			if err := s.processRequest(); err != nil {
				log.Errorf("failed to process request: %s", err)
			}
		}
	}
}

func (s *Service) processRequest() error {
	var request Request
	if err := s.db.First(&request, "status = ?", "created").Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("failed to get request: %w", err)
	}
	start := time.Now()
	log.Infow("processing request", "id", request.ID,
		"miner", request.Miner.Address, "from", request.From, "to", request.To,
		"extension", request.Extension, "new_expiration", request.NewExpiration,
		"dry_run", request.DryRun)

	fromEpoch := TimestampToEpoch(request.From.Unix())
	toEpoch := TimestampToEpoch(request.To.Unix())

	messages, dryRuns, err := s.extend(context.Background(), request.Miner.Address,
		fromEpoch, toEpoch, request.Extension, request.NewExpiration, tolerance, request.DryRun)
	if err != nil {
		log.Errorf("failed to extend: %s", err)
		request.Status = RequestStatusFailed
		request.Error = err.Error()
	} else {
		log.Infof("successfully extended %d sectors", len(messages)+len(dryRuns))
		if len(messages) == 0 && len(dryRuns) == 0 {
			request.Status = RequestStatusSuccess
		} else {
			if request.DryRun {
				request.Status = RequestStatusSuccess
				b, err := json.MarshalIndent(dryRuns, "", "  ")
				if err != nil {
					log.Errorf("failed to marshal dry runs: %s", err)
				} else {
					request.DryRunResult = string(b)
				}
			} else {
				request.Status = RequestStatusPending
				request.Messages = messages
			}
		}
	}
	request.Took = time.Since(start)
	if err := s.db.Save(&request).Error; err != nil {
		log.Errorf("failed to save request: %s", err)
	}
	return nil
}

func (s *Service) createRequest(ctx context.Context, minerAddr address.Address, from, to time.Time, extension, newExpiration *abi.ChainEpoch, dryRun bool) (*Request, error) {
	if extension == nil && newExpiration == nil {
		return nil, fmt.Errorf("either extension or new_expiration must be set")
	}
	if to.Unix() < from.Unix() {
		return nil, fmt.Errorf("to time must be greater than from time")
	}

	head, err := s.api.ChainHead(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain head: %w", err)
	}

	if extension != nil {
		if int(*extension) < tolerance {
			return nil, fmt.Errorf("extension must be greater than %d epochs", tolerance)
		}
	}
	if newExpiration != nil {
		if *newExpiration-head.Height() < tolerance {
			return nil, fmt.Errorf("new expiration must be greater than %d epochs from now", tolerance)
		}
	}

	request := &Request{
		Miner:         Address{minerAddr},
		From:          from,
		To:            to,
		Extension:     extension,
		NewExpiration: newExpiration,
		Status:        RequestStatusCreated,
		DryRun:        dryRun,
	}

	return request, s.db.Create(request).Error
}

func (s *Service) getRequest(ctx context.Context, id uint) (*Request, error) {
	var request Request
	if err := s.db.First(&request, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("request not found")
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}
	return &request, nil
}

func (s *Service) extend(ctx context.Context, addr address.Address, from, to abi.ChainEpoch,
	extension *abi.ChainEpoch, newExpiration *abi.ChainEpoch, tolerance abi.ChainEpoch, dryRun bool) ([]*Message, []*PseudoExtendSectorExpirationParams, error) {

	if extension == nil && newExpiration == nil {
		return nil, nil, fmt.Errorf("either extension or new expiration must be set")
	}

	head, err := s.api.ChainHead(ctx)
	if err != nil {
		return nil, nil, err
	}
	currEpoch := head.Height()

	nv, err := s.api.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network version: %w", err)
	}
	sectorsMax, err := policy.GetAddressedSectorsMax(nv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get addressed sectors max: %w", err)
	}

	// this maybe takes long time based on the number of sectors
	activeSet, err := s.api.StateMinerActiveSectors(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get active sector set: %w", err)
	}

	var sectors []abi.SectorNumber
	activeSectorsInfo := make(map[abi.SectorNumber]*miner.SectorOnChainInfo, len(activeSet))
	for _, info := range activeSet {
		activeSectorsInfo[info.SectorNumber] = info
		if info.Expiration >= from && info.Expiration <= to {
			sectors = append(sectors, info.SectorNumber)
		}
	}

	mAct, err := s.api.StateGetActor(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get miner actor: %w", err)
	}
	mas, err := miner.Load(s.adtStore, mAct)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load miner actor state: %w", err)
	}
	activeSectorsLocation := make(map[abi.SectorNumber]*miner.SectorLocation, len(activeSet))
	if err := mas.ForEachDeadline(func(dlIdx uint64, dl miner.Deadline) error {
		return dl.ForEachPartition(func(partIdx uint64, part miner.Partition) error {
			pas, err := part.ActiveSectors()
			if err != nil {
				return err
			}
			return pas.ForEach(func(i uint64) error {
				activeSectorsLocation[abi.SectorNumber(i)] = &miner.SectorLocation{
					Deadline:  dlIdx,
					Partition: partIdx,
				}
				return nil
			})
		})
	}); err != nil {
		return nil, nil, err
	}

	maxExtension, err := policy.GetMaxSectorExpirationExtension(nv)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get max extension: %w", err)
	}

	extensions := map[miner.SectorLocation]map[abi.ChainEpoch][]abi.SectorNumber{}
	for _, si := range activeSectorsInfo {
		var newExp abi.ChainEpoch
		if extension != nil {
			newExp = si.Expiration + *extension
		}
		if newExpiration != nil {
			newExp = *newExpiration
		}
		maxExtendNow := currEpoch + maxExtension
		if newExp > maxExtendNow {
			newExp = maxExtendNow
		}

		maxExp := si.Activation + policy.GetSectorMaxLifetime(si.SealProof, nv)
		if newExp > maxExp {
			newExp = maxExp
		}
		if newExp <= si.Expiration || withinTolerance(tolerance)(newExp, si.Expiration) {
			continue
		}
		l, found := activeSectorsLocation[si.SectorNumber]
		if !found {
			return nil, nil, fmt.Errorf("location for sector %d not found", si.SectorNumber)
		}
		es, found := extensions[*l]
		if !found {
			ne := make(map[abi.ChainEpoch][]abi.SectorNumber)
			ne[newExp] = []abi.SectorNumber{si.SectorNumber}
			extensions[*l] = ne
		} else {
			added := false
			for exp := range es {
				if withinTolerance(tolerance)(newExp, exp) {
					es[exp] = append(es[exp], si.SectorNumber)
					added = true
					break
				}
			}
			if !added {
				es[newExp] = []abi.SectorNumber{si.SectorNumber}
			}
		}
	}

	verifregAct, err := s.api.StateGetActor(ctx, builtin.VerifiedRegistryActorAddr, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup verifreg actor: %w", err)
	}

	verifregSt, err := verifreg.Load(s.adtStore, verifregAct)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load verifreg state: %w", err)
	}

	claimsMap, err := verifregSt.GetClaims(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup claims for miner: %w", err)
	}

	claimIdsBySector, err := verifregSt.GetClaimIdsBySector(addr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to lookup claim IDs by sector: %w", err)
	}
	declMax, err := policy.GetDeclarationsMax(nv)
	if err != nil {
		return nil, nil, err
	}
	var params []miner.ExtendSectorExpiration2Params
	var cannotExtendSectors []abi.SectorNumber
	p := miner.ExtendSectorExpiration2Params{}
	scount := 0
	for l, exts := range extensions {
		for newExp, numbers := range exts {
			sectorsWithoutClaimsToExtend := bitfield.New()
			var sectorsWithClaims []miner.SectorClaim
			for _, sectorNumber := range numbers {
				claimIdsToMaintain := make([]verifreg.ClaimId, 0)
				claimIdsToDrop := make([]verifreg.ClaimId, 0)
				cannotExtendSector := false
				claimIds, ok := claimIdsBySector[sectorNumber]
				// Nothing to check, add to ccSectors
				if !ok {
					sectorsWithoutClaimsToExtend.Set(uint64(sectorNumber))
				} else {
					for _, claimId := range claimIds {
						claim, ok := claimsMap[claimId]
						if !ok {
							return nil, nil, fmt.Errorf("failed to find claim for claimId %d", claimId)
						}
						claimExpiration := claim.TermStart + claim.TermMax
						// can be maintained in the extended sector
						if claimExpiration > newExp {
							claimIdsToMaintain = append(claimIdsToMaintain, claimId)
						} else {
							log.Infof("skipping sector %d because claim %d does not live long enough", sectorNumber, claimId)
							cannotExtendSector = true
							break
						}
					}
					if cannotExtendSector {
						cannotExtendSectors = append(cannotExtendSectors, sectorNumber)
						continue
					}

					if len(claimIdsToMaintain)+len(claimIdsToDrop) != 0 {
						sectorsWithClaims = append(sectorsWithClaims, miner.SectorClaim{
							SectorNumber:   sectorNumber,
							MaintainClaims: claimIdsToMaintain,
							DropClaims:     claimIdsToDrop,
						})
					}
				}
			}

			sectorsWithoutClaimsCount, err := sectorsWithoutClaimsToExtend.Count()
			if err != nil {
				return nil, nil, fmt.Errorf("failed to count cc sectors: %w", err)
			}

			sectorsInDecl := int(sectorsWithoutClaimsCount) + len(sectorsWithClaims)
			scount += sectorsInDecl

			if scount > sectorsMax || len(p.Extensions) >= declMax {
				params = append(params, p)
				p = miner.ExtendSectorExpiration2Params{}
				scount = sectorsInDecl
			}

			p.Extensions = append(p.Extensions, miner.ExpirationExtension2{
				Deadline:          l.Deadline,
				Partition:         l.Partition,
				Sectors:           SectorNumsToBitfield(numbers),
				SectorsWithClaims: sectorsWithClaims,
				NewExpiration:     newExp,
			})
		}
	}
	// if we have any sectors, then one last append is needed here
	if scount != 0 {
		params = append(params, p)
	}

	if len(params) == 0 {
		log.Info("nothing to extend")
		return nil, nil, nil
	}

	mi, err := s.api.StateMinerInfo(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, nil, fmt.Errorf("getting miner info: %w", err)
	}

	stotal := 0
	var messages []*Message
	var dryRuns []*PseudoExtendSectorExpirationParams
	for i := range params {
		scount := 0
		for _, ext := range params[i].Extensions {
			count, err := ext.Sectors.Count()
			if err != nil {
				return nil, nil, err
			}
			scount += int(count)
		}
		log.Infof("extending %d sectors", scount)
		stotal += scount

		if dryRun {
			pp, err := NewPseudoExtendParams(&params[i])
			if err != nil {
				return nil, nil, err
			}
			dryRuns = append(dryRuns, pp)
			continue
		}

		sp, aerr := actors.SerializeParams(&params[i])
		if aerr != nil {
			return nil, nil, fmt.Errorf("serializing params: %w", err)
		}

		smsg, err := s.api.MpoolPushMessage(ctx, &types.Message{
			From:   mi.Worker,
			To:     addr,
			Method: builtin.MethodsMiner.ExtendSectorExpiration2,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("mpool push message: %w", err)
		}
		log.Infow("pushed extend message", "cid", smsg.Cid(), "to", addr, "from", mi.Worker, "sectors", scount)
		exts, err := NewExtension2FromParams(params[i])
		if err != nil {
			log.Errorf("creating extension2 from params: %s", err)
		}

		msg := &Message{
			Cid:        CID{smsg.Cid()},
			Extensions: exts,
		}
		s.db.Create(msg)
		messages = append(messages, msg)
	}
	return messages, dryRuns, nil
}

func SectorNumsToBitfield(sectors []abi.SectorNumber) bitfield.BitField {
	var numbers []uint64
	for _, sector := range sectors {
		numbers = append(numbers, uint64(sector))
	}

	return bitfield.NewFromSet(numbers)
}

type PseudoExtendSectorExpirationParams struct {
	Extensions []PseudoExpirationExtension
}

type PseudoExpirationExtension struct {
	Deadline      uint64
	Partition     uint64
	Sectors       string
	NewExpiration abi.ChainEpoch
}

func NewPseudoExtendParams(p *miner.ExtendSectorExpiration2Params) (*PseudoExtendSectorExpirationParams, error) {
	res := PseudoExtendSectorExpirationParams{}
	for _, ext := range p.Extensions {
		scount, err := ext.Sectors.Count()
		if err != nil {
			return nil, err
		}

		sectors, err := ext.Sectors.All(scount)
		if err != nil {
			return nil, err
		}

		res.Extensions = append(res.Extensions, PseudoExpirationExtension{
			Deadline:      ext.Deadline,
			Partition:     ext.Partition,
			Sectors:       ArrayToString(sectors),
			NewExpiration: ext.NewExpiration,
		})
	}
	return &res, nil
}

// ArrayToString Example: {1,3,4,5,8,9} -> "1,3-5,8-9"
func ArrayToString(array []uint64) string {
	sort.Slice(array, func(i, j int) bool {
		return array[i] < array[j]
	})

	var sarray []string
	s := ""

	for i, elm := range array {
		if i == 0 {
			s = strconv.FormatUint(elm, 10)
			continue
		}
		if elm == array[i-1] {
			continue // filter out duplicates
		} else if elm == array[i-1]+1 {
			s = strings.Split(s, "-")[0] + "-" + strconv.FormatUint(elm, 10)
		} else {
			sarray = append(sarray, s)
			s = strconv.FormatUint(elm, 10)
		}
	}

	if s != "" {
		sarray = append(sarray, s)
	}

	return strings.Join(sarray, ",")
}

func withinTolerance(t abi.ChainEpoch) func(a, b abi.ChainEpoch) bool {
	return func(a, b abi.ChainEpoch) bool {
		diff := a - b
		if diff < 0 {
			diff = -diff
		}
		return diff <= t
	}
}
