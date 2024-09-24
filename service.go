package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-bitfield"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/blockstore"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/lotus/chain/actors/adt"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/filecoin-project/lotus/chain/actors/builtin/verifreg"
	"github.com/filecoin-project/lotus/chain/actors/policy"
	"github.com/filecoin-project/lotus/chain/messagepool"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/filecoin-project/lotus/node/config"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultTolerance = abi.ChainEpoch(20160) // default tolerance is 7 days
)

type watchMessage struct {
	id       uint
	cid      CID
	started  time.Time
	cancelCh chan struct{}
}

func newWatchMessage(m *Message) *watchMessage {
	return &watchMessage{
		id:       m.ID,
		cid:      m.Cid,
		started:  time.Now(),
		cancelCh: make(chan struct{}),
	}
}

func (w *watchMessage) Cancel() {
	close(w.cancelCh)
}

type Service struct {
	api              API
	adtStore         adt.Store
	db               *gorm.DB
	maxWait          time.Duration
	wg               sync.WaitGroup
	watchingMessages *SafeMap[uint, *watchMessage]
	shutdownFunc     context.CancelFunc
}

func NewService(ctx context.Context, db *gorm.DB, api API, maxWait time.Duration) *Service {
	tbs := blockstore.NewTieredBstore(blockstore.NewAPIBlockstore(api), blockstore.NewMemory())
	adtStore := adt.WrapStore(ctx, cbor.NewCborStore(tbs))

	ctx, cancel := context.WithCancel(ctx)

	s := &Service{
		db:               db,
		api:              api,
		adtStore:         adtStore,
		watchingMessages: NewSafeMap[uint, *watchMessage](),
		maxWait:          maxWait,
		shutdownFunc: func() {
			cancel()
		},
	}
	s.wg.Add(3)
	go s.runProcessor(ctx)
	go s.runMessageChecker(ctx)
	go s.runPendingChecker(ctx)
	return s
}

func (s *Service) Shutdown(_ context.Context) error {
	log.Info("waiting for services to shutdown")

	if err := s.watchingMessages.Range(func(k uint, wm *watchMessage) error {
		wm.Cancel()
		return nil
	}); err != nil {
		log.Errorf("failed to cancel watching message: %s", err)
	}

	s.shutdownFunc()
	s.wg.Wait()
	return nil
}

func (s *Service) runProcessor(ctx context.Context) {
	defer s.wg.Done()
	log.Info("starting extend processor")
	tk := time.NewTicker(5 * time.Second)
	defer tk.Stop()

	processingRequests := NewSafeMap[uint, struct{}]()
	for {
		select {
		case <-ctx.Done():
			log.Info("context done, stopping extend processor")
			return
		case <-tk.C:
			var requests []*Request
			if err := s.db.Preload(clause.Associations).
				Where("status = ?", RequestStatusCreated).
				Find(&requests).Error; err != nil {
				log.Errorf("failed to get request: %s", err)
				continue
			}
			if len(requests) == 0 {
				continue
			}
			log.Debugf("found %d new created requests", len(requests))
			for _, request := range requests {
				if processingRequests.Has(request.ID) {
					continue
				}
				processingRequests.Set(request.ID, struct{}{})
				go func(request *Request) {
					defer processingRequests.Delete(request.ID)
					if err := s.processRequest(ctx, request); err != nil {
						log.Errorf("failed to process request %d: %s", request.ID, err)
					}
				}(request)
			}
		}
	}
}

func (s *Service) runMessageChecker(ctx context.Context) {
	defer s.wg.Done()
	log.Info("starting message checker")
	tk := time.NewTicker(builtin.EpochDurationSeconds * time.Second) // check interval is 30 seconds
	defer tk.Stop()

	checkingRequests := NewSafeMap[uint, struct{}]()
	for {
		select {
		case <-ctx.Done():
			log.Info("context done, stopping message checker")
			return
		case <-tk.C:
			var requests []*Request
			if err := s.db.Preload(clause.Associations).
				Where("status = ?", RequestStatusPending).
				Find(&requests).Error; err != nil {
				log.Errorf("failed to get request: %s", err)
				continue
			}
			if len(requests) == 0 {
				continue
			}
			log.Debugf("found %d pending requests", len(requests))

			for _, request := range requests {
				if checkingRequests.Has(request.ID) {
					continue
				}
				checkingRequests.Set(request.ID, struct{}{})
				go func(request *Request) {
					defer checkingRequests.Delete(request.ID)
					if err := s.checkMessage(ctx, request); err != nil {
						log.Errorf("failed to process request %d: %s", request.ID, err)
					}
				}(request)
			}
		}
	}
}

func (s *Service) processRequest(ctx context.Context, request *Request) error {
	sLog := log.With("id", request.ID)

	sLog.Infow("processing request",
		"miner", request.Miner.Address, "from", request.From, "to", request.To,
		"extension", request.Extension, "new_expiration", request.NewExpiration,
		"tolerance", request.Tolerance, "dry_run", request.DryRun)
	fromEpoch := TimestampToEpoch(request.From)
	toEpoch := TimestampToEpoch(request.To)

	start := time.Now()

	result, terr := s.extend(ctx, request.Miner.Address, fromEpoch, toEpoch,
		request.Extension, request.NewExpiration, request.Tolerance,
		request.MaxSectors, abi.TokenAmount(types.MustParseFIL(fmt.Sprintf("%d", request.MaxInitialPledges))), request.DryRun, sLog)

	if terr != nil {
		request.Error = terr.Error()
	}
	if (result == nil) || len(result.Messages)+len(result.DryRuns) == 0 {
		if terr == nil {
			request.Status = RequestStatusSuccess // no sectors need to extend
		} else {
			sLog.Errorf("processing request failed: %s, took: %s", terr, time.Since(start))
			request.Status = RequestStatusFailed
		}
	} else {
		request.PublishedSectors = result.Published
		request.TotalSectors = result.TotalSectors

		if request.DryRun {
			b, err := json.MarshalIndent(result.DryRuns, "", "  ")
			if err != nil {
				sLog.Errorf("failed to marshal dry runs: %s", err)
			} else {
				request.DryRunResult = string(b)
			}
			if terr == nil {
				request.Status = RequestStatusSuccess
			} else {
				request.Status = RequestStatusPartfailed
			}
		} else {
			// even if some messages are failed, we still make the request status to pending
			// handle the failed messages in the message checker
			request.Status = RequestStatusPending
			request.Messages = result.Messages
		}
	}
	sLog.Infow("processing request", "status", request.Status, "took", time.Since(start))
	request.Took = time.Since(start).Seconds()
	if err := s.db.Save(&request).Error; err != nil {
		sLog.Errorf("failed to save request: %s", err)
	}
	return nil
}

func (s *Service) createRequest(ctx context.Context, minerAddr address.Address, from, to time.Time,
	extension, newExpiration, tolerance *abi.ChainEpoch, maxSectors int, maxInitialPledges int, dryRun bool) (*Request, error) {
	if extension == nil && newExpiration == nil {
		return nil, fmt.Errorf("either extension or new_expiration must be set")
	}
	if to.Unix() < from.Unix() {
		return nil, fmt.Errorf("to time must be greater than from time")
	}

	if from.Before(time.Now().Add(time.Hour)) {
		return nil, fmt.Errorf("from time must be at least 1 hour in the future")
	}

	var tol = defaultTolerance
	if tolerance != nil {
		tol = *tolerance
	}

	if extension != nil {
		if (*extension) < tol {
			return nil, fmt.Errorf("extension must be greater than %d epochs", tolerance)
		}
	}
	if newExpiration != nil {
		tol = 0
	}

	nv, err := s.api.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to get network version: %w", err)
	}
	sectorsMax, err := policy.GetAddressedSectorsMax(nv)
	if err != nil {
		return nil, fmt.Errorf("failed to get addressed sectors max: %w", err)
	}
	if maxSectors > sectorsMax {
		return nil, fmt.Errorf("max sectors must be less than %d", sectorsMax)
	}

	request := &Request{
		Miner:             Address{minerAddr},
		From:              from,
		To:                to,
		Extension:         extension,
		NewExpiration:     newExpiration,
		Tolerance:         tol,
		Status:            RequestStatusCreated,
		MaxSectors:        maxSectors,
		MaxInitialPledges: maxInitialPledges,
		DryRun:            dryRun,
	}

	return request, s.db.Create(request).Error
}

func (s *Service) getRequest(_ context.Context, id uint) (*Request, error) {
	var request Request
	if err := s.db.Preload(clause.Associations).First(&request, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("request not found")
		}
		return nil, fmt.Errorf("failed to get request: %w", err)
	}
	return &request, nil
}

func (s *Service) extend(ctx context.Context, addr address.Address, from, to abi.ChainEpoch,
	extension *abi.ChainEpoch, newExpiration *abi.ChainEpoch, tolerance abi.ChainEpoch,
	maxSectors int, maxInitialPledge abi.TokenAmount, dryRun bool, sLog *zap.SugaredLogger) (*extendResult, error) {

	if extension == nil && newExpiration == nil {
		return nil, fmt.Errorf("either extension or new expiration must be set")
	}

	sLog.Infow("extending sectors", "miner", addr, "from", from, "to", to, "extension", extension, "new_expiration", newExpiration, "dry_run", dryRun)

	head, err := s.api.ChainHead(ctx)
	if err != nil {
		return nil, err
	}
	currEpoch := head.Height()

	nv, err := s.api.StateNetworkVersion(ctx, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to get network version: %w", err)
	}
	sectorsMax, err := policy.GetAddressedSectorsMax(nv)
	if err != nil {
		return nil, fmt.Errorf("failed to get addressed sectors max: %w", err)
	}

	addrSectors := sectorsMax
	if maxSectors != 0 {
		addrSectors = maxSectors
		if addrSectors > sectorsMax {
			return nil, fmt.Errorf("the specified max-sectors exceeds the maximum limit")
		}
	}

	time1 := time.Now()
	activeSet, err := warpActiveSectors(ctx, s.api, addr, lo.If(os.Getenv("CACHE_ACTIVE_SECTORS") == "1", true).Else(false)) // only for debug, do not cache in production
	if err != nil {
		return nil, fmt.Errorf("failed to get active set: %w", err)
	}
	sLog.Infow("got active set", "took", time.Since(time1), "sectors", len(activeSet))

	type sectorInfo struct {
		sectorNumber  abi.SectorNumber
		initialPledge abi.TokenAmount
	}

	var sectors []sectorInfo
	activeSectorsInfo := make(map[abi.SectorNumber]*miner.SectorOnChainInfo, len(activeSet))
	for _, info := range activeSet {
		if info.Expiration >= from && info.Expiration <= to {
			activeSectorsInfo[info.SectorNumber] = info
			sectors = append(sectors, sectorInfo{
				sectorNumber:  info.SectorNumber,
				initialPledge: info.InitialPledge,
			})
		}
	}

	sort.Slice(sectors, func(i, j int) bool {
		return sectors[i].initialPledge.LessThan(sectors[j].initialPledge)
	})

	sLog.Infof("found %d sectors to extend", len(sectors))
	if len(sectors) == 0 {
		sLog.Info("nothing to extend, break")
		return &extendResult{}, nil
	}

	mAct, err := s.api.StateGetActor(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to get miner actor: %w", err)
	}
	mas, err := miner.Load(s.adtStore, mAct)
	if err != nil {
		return nil, fmt.Errorf("failed to load miner actor state: %w", err)
	}
	time2 := time.Now()
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
		return nil, err
	}
	sLog.Infof("got active sectors location, took: %s", time.Since(time2))

	maxExtension, err := policy.GetMaxSectorExpirationExtension(nv)
	if err != nil {
		return nil, fmt.Errorf("failed to get max extension: %w", err)
	}

	totalPledge := abi.NewTokenAmount(0)
	extensions := map[miner.SectorLocation]map[abi.ChainEpoch][]abi.SectorNumber{}
	for _, sc := range sectors {
		ntPledge := big.Add(totalPledge, sc.initialPledge)
		if !maxInitialPledge.NilOrZero() && ntPledge.GreaterThan(maxInitialPledge) {
			// We have reached the max initial pledge
			sLog.Infof("total pledge %s exceeds max initial pledge %s", ntPledge, maxInitialPledge)
			continue
		}
		totalPledge = ntPledge
		si := activeSectorsInfo[sc.sectorNumber]

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
			return nil, fmt.Errorf("location for sector %d not found", si.SectorNumber)
		}
		sLog.Debugf("extending sector %d from %d to %d", si.SectorNumber, si.Expiration, newExp)
		es, found := extensions[*l]
		if !found {
			sLog.Debugw(si.SectorNumber.String(), "found", found, "exp", si.Expiration, "newExp", newExp)
			ne := make(map[abi.ChainEpoch][]abi.SectorNumber)
			ne[newExp] = []abi.SectorNumber{si.SectorNumber}
			extensions[*l] = ne
		} else {
			added := false
			for exp := range es {
				if withinTolerance(tolerance)(newExp, exp) {
					es[exp] = append(es[exp], si.SectorNumber)
					added = true
					sLog.Debugw(si.SectorNumber.String(), "tolerance", tolerance, "exp", si.Expiration, "newExp", exp)
					break
				}
			}
			if !added {
				sLog.Debugw(si.SectorNumber.String(), "exp", si.Expiration, "newExp", newExp)
				es[newExp] = []abi.SectorNumber{si.SectorNumber}
			}
		}
	}

	verifregAct, err := s.api.StateGetActor(ctx, builtin.VerifiedRegistryActorAddr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup verifreg actor: %w", err)
	}

	verifregSt, err := verifreg.Load(s.adtStore, verifregAct)
	if err != nil {
		return nil, fmt.Errorf("failed to load verifreg state: %w", err)
	}

	claimsMap, err := verifregSt.GetClaims(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup claims for miner: %w", err)
	}

	claimIdsBySector, err := verifregSt.GetClaimIdsBySector(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup claim IDs by sector: %w", err)
	}
	declMax, err := policy.GetDeclarationsMax(nv)
	if err != nil {
		return nil, err
	}
	var params []miner.ExtendSectorExpiration2Params
	var cannotExtendSectors []abi.SectorNumber
	p := miner.ExtendSectorExpiration2Params{}
	scount := 0
	d1 := 0
	for l, exts := range extensions {
		for newExp, numbers := range exts {
			d1 += len(numbers)
			sLog.Debugf("extending sectors for partition %d-%d, extend %d sectors to %d", l.Deadline, l.Partition, len(numbers), newExp)
			for len(numbers) > addrSectors {
				var currentNumbers []abi.SectorNumber
				currentNumbers, numbers = numbers[:addrSectors], numbers[addrSectors:]
				e2, ce, _, err := buildParams(l, newExp, currentNumbers, claimIdsBySector, claimsMap)
				if err != nil {
					return nil, fmt.Errorf("failed to build params: %w", err)
				}
				if len(ce) != 0 {
					cannotExtendSectors = append(cannotExtendSectors, ce...)
				}
				p1 := miner.ExtendSectorExpiration2Params{}
				p1.Extensions = append(p1.Extensions, *e2)
				params = append(params, p1)
			}
			if len(numbers) > 0 {
				e2, ce, sectorsInDecl, err := buildParams(l, newExp, numbers, claimIdsBySector, claimsMap)
				if err != nil {
					return nil, fmt.Errorf("failed to build params: %w", err)
				}
				if len(ce) != 0 {
					cannotExtendSectors = append(cannotExtendSectors, ce...)
				}
				scount += sectorsInDecl
				if scount > addrSectors || len(p.Extensions) >= declMax {
					params = append(params, p)
					p = miner.ExtendSectorExpiration2Params{}
					scount = sectorsInDecl
				}
				p.Extensions = append(p.Extensions, *e2)
			}
		}
	}
	sLog.Infof("total %d sectors to extend, %d cannot extend", d1, len(cannotExtendSectors))
	// if we have any sectors, then one last append is needed here
	if scount != 0 {
		params = append(params, p)
	}
	if len(params) == 0 {
		sLog.Info("nothing to extend")
		return &extendResult{
			TotalSectors: len(sectors),
		}, nil
	}

	mi, err := s.api.StateMinerInfo(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("getting miner info: %w", err)
	}
	stotal := 0
	published := 0
	var messages []*Message
	var dryRuns []*PseudoExtendSectorExpirationParams
	var errMsgs []string
loopParams:
	for i := range params {
		scount := 0
		for _, ext := range params[i].Extensions {
			count, err := ext.Sectors.Count()
			if err != nil {
				errMsgs = append(errMsgs, err.Error())
				continue loopParams
			}
			scount += int(count)
		}
		sLog.Infof("extending %d sectors on message index: %d", scount, i)
		stotal += scount

		if dryRun {
			pp, err := NewPseudoExtendParams(&params[i])
			if err != nil {
				errMsgs = append(errMsgs, err.Error())
				continue
			}
			dryRuns = append(dryRuns, pp)
			published += scount
			continue
		}

		sp, aerr := actors.SerializeParams(&params[i])
		if aerr != nil {
			errMsgs = append(errMsgs, fmt.Errorf("serializing params: %w", err).Error())
			continue
		}

		smsg, err := s.api.MpoolPushMessage(ctx, &types.Message{
			From:   mi.Worker,
			To:     addr,
			Method: builtin.MethodsMiner.ExtendSectorExpiration2,
			Value:  big.Zero(),
			Params: sp,
		}, nil)
		if err != nil {
			sLog.Errorf("failed to push message %d: %s", i, err)
			errMsgs = append(errMsgs, fmt.Errorf("mpool push message: %w", err).Error())
			continue
		}
		published += scount
		sLog.Infow("pushed extend message", "cid", smsg.Cid(), "to", addr, "from", mi.Worker, "sectors", scount)
		exts, err := NewExtension2FromParams(params[i])
		if err != nil {
			sLog.Errorf("creating extension2 from params: %s", err)
		}

		msg := &Message{
			Cid:        CID{smsg.Cid()},
			Extensions: exts,
			Sectors:    scount,
		}
		s.db.Create(msg)
		messages = append(messages, msg)
	}
	if stotal != len(sectors) {
		sLog.Warnw("not all sectors are build to extend", "total", len(sectors), "extended", stotal, "maxInitialPledge", types.FIL(maxInitialPledge).String())
	} else {
		sLog.Infof("all sectors are build to extend: %d", stotal)
	}
	if published != len(sectors) {
		sLog.Warnw("not all sectors are published", "total", len(sectors), "published", published)
	} else {
		sLog.Infof("all sectors are published: %d", published)
	}

	result := &extendResult{
		Messages:     messages,
		DryRuns:      dryRuns,
		TotalSectors: len(sectors),
		Published:    published,
	}
	if len(errMsgs) == 0 {
		return result, nil
	}
	// join errors as one error
	return result, fmt.Errorf("%s", strings.Join(errMsgs, ";\n"))
}

type extendResult struct {
	Messages     []*Message
	DryRuns      []*PseudoExtendSectorExpirationParams
	TotalSectors int
	Published    int
}

func buildParams(l miner.SectorLocation, newExp abi.ChainEpoch, numbers []abi.SectorNumber, claimIdsBySector map[abi.SectorNumber][]verifreg.ClaimId, claimsMap map[verifreg.ClaimId]verifreg.Claim) (*miner.ExpirationExtension2, []abi.SectorNumber, int, error) {
	sectorsWithoutClaimsToExtend := bitfield.New()
	var sectorsWithClaims []miner.SectorClaim
	var cannotExtendSectors []abi.SectorNumber
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
					return nil, nil, 0, fmt.Errorf("failed to find claim for claimId %d", claimId)
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
		return nil, nil, 0, fmt.Errorf("failed to count cc sectors: %w", err)
	}
	sectorsInDecl := int(sectorsWithoutClaimsCount) + len(sectorsWithClaims)
	e2 := miner.ExpirationExtension2{
		Deadline:          l.Deadline,
		Partition:         l.Partition,
		Sectors:           SectorNumsToBitfield(numbers),
		SectorsWithClaims: sectorsWithClaims,
		NewExpiration:     newExp,
	}
	return &e2, cannotExtendSectors, sectorsInDecl, nil
}

func (s *Service) speedupRequest(ctx context.Context, id uint, mss *api.MessageSendSpec) error {
	var request Request
	if err := s.db.Preload(clause.Associations).First(&request, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("request not found")
		}
		return fmt.Errorf("failed to get request: %w", err)
	}
	if request.Status != RequestStatusPending {
		return fmt.Errorf("request is not pending")
	}

	for _, msg := range request.Messages {
		if msg.OnChain {
			continue
		}
		// todo: need order by nonce?
		if err := s.replaceMessage(ctx, msg.ID, mss); err != nil {
			return fmt.Errorf("failed to replace message: %w", err)
		}
	}
	return nil
}

func (s *Service) checkMessage(ctx context.Context, request *Request) error {
	sLog := log.With("request", request.ID)
	sLog.Debugw("check request messages",
		"miner", request.Miner.Address, "message count", len(request.MessageCids()))

	var allOnChain = true
	var allSuccess = true
	var errorMsgs []string
	for _, msg := range request.Messages {
		if msg.OnChain {
			// watch message done
			if !errors.Is(msg.ExitCode, exitcode.Ok) {
				allSuccess = false
				errorMsgs = append(errorMsgs, msg.ExitCode.Error())
			}
			continue
		}
		allOnChain = false

		if s.watchingMessages.Has(msg.ID) {
			// still watching
			continue
		}
		go s.watchMessage(ctx, msg.ID)
	}
	if allOnChain {
		request.ConfirmedAt = lo.ToPtr(time.Now())
		if allSuccess {
			if len(request.Error) == 0 {
				request.Status = RequestStatusSuccess
				sLog.Infof("request all messages on chain, status: %s", request.Status)
			} else {
				request.Status = RequestStatusPartfailed
				sLog.Infof("request all messages on chain, but got preceding error: %s", request.Error)
			}
		} else {
			if len(errorMsgs) == len(request.Messages) {
				// all failed
				request.Status = RequestStatusFailed
			} else {
				request.Status = RequestStatusPartfailed
			}
			if len(request.Error) != 0 {
				errorMsgs = append([]string{request.Error}, errorMsgs...)
			}
			request.Error = strings.Join(errorMsgs, ";")
			sLog.Infof("messages on chain, status: %s, failed: %d", request.Status, len(errorMsgs))
		}
		if err := s.db.Save(&request).Error; err != nil {
			return fmt.Errorf("failed to save request: %w", err)
		}
	}
	return nil
}

func (s *Service) watchMessage(ctx context.Context, id uint) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sLog := log.With("message", id)

	var msg Message
	if err := s.db.First(&msg, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sLog.Error("message not found")
		} else {
			sLog.Errorf("failed to get message: %s", err)
		}
		return
	}
	sLog = sLog.With("cid", msg.Cid.String(), "request", msg.RequestID)

	if s.watchingMessages.Has(msg.ID) {
		sLog.Warn("message is already watching")
		return
	}
	wm := newWatchMessage(&msg)
	s.watchingMessages.Set(msg.ID, wm)
	defer s.watchingMessages.Delete(msg.ID)

	sLog.Info("watching message")

	resultChan := make(chan *api.MsgLookup, 1)
	errorChan := make(chan error, 1)

	go func() {
		receipt, err := s.api.StateWaitMsg(ctx, msg.Cid.Cid, 2*build.MessageConfidence, api.LookbackNoLimit, true)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- receipt
	}()

	select {
	case receipt := <-resultChan:
		sLog.Info("message on chain")
		msg.OnChain = true
		msg.ExitCode = receipt.Receipt.ExitCode
		msg.Return = receipt.Receipt.Return
		msg.GasUsed = receipt.Receipt.GasUsed
		if err := s.db.Save(&msg).Error; err != nil {
			sLog.Errorf("failed to save message: %s", err)
		}
	case err := <-errorChan:
		sLog.Errorf("failed to wait message: %s", err)
	case <-ctx.Done():
		sLog.Info("context done, stopping watching message")
	case <-wm.cancelCh:
		sLog.Info("cancel watching message")
	}
}

func (s *Service) runPendingChecker(ctx context.Context) {
	defer s.wg.Done()
	log.Info("starting pending checker")
	tk := time.NewTicker(builtin.EpochDurationSeconds * time.Second) // check interval is 30 seconds
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Info("context done, stopping pending checker")
			return
		case <-tk.C:
			func() {
				var replaceMessages []uint
				err := s.watchingMessages.Range(func(k uint, wm *watchMessage) error {
					if time.Since(wm.started) > 6*time.Hour {
						log.Warnw("message is pending too long", "id", k, "took", time.Since(wm.started))
					}
					if s.maxWait > 0 && time.Since(wm.started) > s.maxWait {
						replaceMessages = append(replaceMessages, k)
					}
					return nil
				})
				if err != nil {
					log.Errorf("failed to range watching messages: %s", err)
				}

				maxFee, _ := types.ParseFIL("1FIL") // todo: get from config

				mss := &api.MessageSendSpec{
					MaxFee: abi.TokenAmount(maxFee),
				}
				for _, id := range replaceMessages {
					if err := s.replaceMessage(ctx, id, mss); err != nil {
						log.Errorf("failed to replace message: %s", err)
					}
				}
			}()
		}
	}
}

func (s *Service) replaceMessage(ctx context.Context, id uint, mss *api.MessageSendSpec) error {
	var m Message
	if err := s.db.Preload(clause.Associations).First(&m, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("message id not found in db: %d", id)
		}
		return fmt.Errorf("failed to get request: %w", err)
	}
	sLog := log.With("request", m.RequestID, "id", id, "cid", m.Cid.String())
	sLog.Info("replacing message")

	// get the message from the chain
	cm, err := s.api.ChainGetMessage(ctx, m.Cid.Cid)
	if err != nil {
		return fmt.Errorf("could not find referenced message: %w", err)
	}

	ts, err := s.api.ChainHead(ctx)
	if err != nil {
		return fmt.Errorf("getting chain head: %w", err)
	}

	pending, err := s.api.MpoolPending(ctx, ts.Key())
	if err != nil {
		return err
	}

	var found *types.SignedMessage
	for _, p := range pending {
		if p.Message.From == cm.From && p.Message.Nonce == cm.Nonce {
			found = p
			break
		}
	}

	// If the message is not found in the mpool, skip it and continue with the next one
	if found == nil {
		sLog.Warn("message not found in mpool, skipping")
		return nil
	}
	msg := found.Message

	cfg, err := s.api.MpoolGetConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to lookup the message pool config: %w", err)
	}

	defaultRBF := messagepool.ComputeRBF(msg.GasPremium, cfg.ReplaceByFeeRatio)

	//msg.GasLimit = 0 // clear gas limit
	msg.GasFeeCap = abi.NewTokenAmount(0)
	msg.GasPremium = abi.NewTokenAmount(0)
	ret, err := s.api.GasEstimateMessageGas(ctx, &msg, mss, types.EmptyTSK)
	if err != nil {
		return fmt.Errorf("failed to estimate gas values: %w", err)
	}
	msg.GasPremium = big.Max(ret.GasPremium, defaultRBF)
	msg.GasFeeCap = big.Max(ret.GasFeeCap, msg.GasPremium)
	//msg.GasLimit = ret.GasLimit // set new gas limit

	mff := func() (abi.TokenAmount, error) {
		return abi.TokenAmount(config.DefaultDefaultMaxFee()), nil
	}
	messagepool.CapGasFee(mff, &msg, mss)

	smsg, err := s.api.WalletSignMessage(ctx, msg.From, &msg)
	if err != nil {
		return fmt.Errorf("failed to sign message: %w", err)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&m).Error; err != nil {
			return err
		}
		newID, err := s.api.MpoolPush(ctx, smsg)
		if err != nil {
			return fmt.Errorf("failed to push new message to mempool: %w", err)
		}

		newMsg := &Message{
			Cid:        CID{newID},
			Extensions: m.Extensions,
			RequestID:  m.RequestID,
			Sectors:    m.Sectors,
		}

		if err := tx.Create(newMsg).Error; err != nil {
			return err
		}
		sLog.Infow("replaced message", "new id", newMsg.ID, "new cid", newID)

		// remove old watching message
		if owm, ok := s.watchingMessages.Get(id); ok {
			owm.Cancel()
		}

		return nil
	})
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

// warpActiveSectors returns active sectors for the miner, cached in a file, if cache is true
// this is for debugging, do not use in production
func warpActiveSectors(ctx context.Context, api API, addr address.Address, cache bool) ([]*miner.SectorOnChainInfo, error) {
	if cache {
		v, err := activeSetFromCache(addr)
		if err == nil {
			log.Warnf("%s using cached active set", addr)
			return v, nil
		}
	}
	// this maybe takes long time based on the number of sectors
	activeSet, err := api.StateMinerActiveSectors(ctx, addr, types.EmptyTSK)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sector set: %w", err)
	}

	defer func() {
		if cache {
			if err := activeSetCacheToFile(addr, activeSet); err != nil {
				log.Errorf("failed to cache active set: %s", err)
			}
		}
	}()
	return activeSet, nil
}

// for debugging
func activeSetCacheToFile(addr address.Address, activeSet []*miner.SectorOnChainInfo) error {
	data, err := json.Marshal(activeSet)
	if err != nil {
		return err
	}

	err = os.WriteFile(fmt.Sprintf("%s.activeSet.json", addr), data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// for debugging
func activeSetFromCache(addr address.Address) ([]*miner.SectorOnChainInfo, error) {
	data, err := os.ReadFile(fmt.Sprintf("%s.activeSet.json", addr))
	if err != nil {
		return nil, err
	}
	var activeSet []*miner.SectorOnChainInfo
	err = json.Unmarshal(data, &activeSet)
	if err != nil {
		return nil, err
	}

	return activeSet, nil
}
