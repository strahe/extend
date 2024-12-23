package main

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/exitcode"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type RequestStatus string

const (
	RequestStatusCreated    RequestStatus = "created"
	RequestStatusPending    RequestStatus = "pending"
	RequestStatusFailed     RequestStatus = "failed"
	RequestStatusPartfailed RequestStatus = "partfailed"
	RequestStatusSuccess    RequestStatus = "success"
)

// Request represents a request in the system.
type Request struct {
	ID                uint            `json:"id" gorm:"primarykey"`
	Miner             Address         `gorm:"index" json:"miner"`
	From              time.Time       `json:"from"`
	To                time.Time       `json:"to"`
	Extension         *abi.ChainEpoch `json:"extension"`
	NewExpiration     *abi.ChainEpoch `json:"new_expiration"`
	Tolerance         abi.ChainEpoch  `json:"tolerance"`
	MaxSectors        int             `json:"max_sectors"`
	MaxInitialPledges float64         `json:"max_initial_pledges"`
	BasefeeLimit      *int64          `json:"basefee_limit"`
	Status            RequestStatus   `gorm:"index" json:"status"`
	Messages          []*Message      `json:"messages"`
	TotalSectors      int             `json:"total_sectors"`
	PublishedSectors  int             `json:"published_sectors"`
	SucceededSectors  int             `json:"succeeded_sectors"`
	Error             string          `json:"error"`
	Took              float64         `json:"took"` // Time in seconds
	ConfirmedAt       *time.Time      `json:"confirmed_at"`
	DryRun            bool            `json:"dry_run"`
	DryRunResult      string          `json:"dry_run_result"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	DeletedAt         gorm.DeletedAt  `gorm:"index" json:"-"`
}

func (r *Request) MessageCids() []string {
	var cids []string
	for _, msg := range r.Messages {
		cids = append(cids, msg.Cid.String())
	}
	return cids
}

func (r *Request) AfterFind(_ *gorm.DB) (err error) {
	if r.DryRun {
		r.SucceededSectors = r.PublishedSectors
	} else {
		var success int
		for _, msg := range r.Messages {
			if msg.OnChain && msg.ExitCode.IsSuccess() {
				success += msg.Sectors
			}
		}
		r.SucceededSectors = success
	}
	return nil
}

func (r *Request) Finished() bool {
	return r.Status == RequestStatusSuccess || r.Status == RequestStatusFailed || r.Status == RequestStatusPartfailed
}

// Message represents a message in the system.
// It contains a unique identifier (MsgCid) and a list of Extensions.
type Message struct {
	ID         uint         `gorm:"primarykey"`
	Cid        CID          `gorm:"index"` // The unique identifier of the message
	Extensions []Extension2 // The list of extensions associated with the message
	Nonce      uint64
	RequestID  uint
	OnChain    bool
	ExitCode   exitcode.ExitCode
	Return     []byte
	GasUsed    int64
	Sectors    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
}

func (m *Message) MarshalJSON() ([]byte, error) {
	// We only need to marshal the CID as a string
	return json.Marshal(m.Cid.String())
}

// Extension2 represents an extension in the system.
// It contains information about the deadline, partition, sectors, sectors with claims, new expiration and message ID.
type Extension2 struct {
	ID                uint           `gorm:"primarykey" json:"id"`
	Deadline          uint64         `json:"deadline"`            // The deadline for the extension
	Partition         uint64         `json:"partition"`           // The partition number
	Sectors           string         `json:"sectors"`             // The sectors associated with the extension
	SectorsWithClaims []byte         `json:"sectors_with_claims"` // The sectors with claims
	NewExpiration     abi.ChainEpoch `json:"new_expiration"`      // The new expiration date for the extension
	MessageID         uint           `json:"message_id"`          // The ID of the message associated with the extension
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// NewExtension2FromParams creates a list of Extension2 from the provided parameters.
// It returns an error if there is an issue with counting the sectors or marshalling the sectors with claims.
func NewExtension2FromParams(params miner.ExtendSectorExpiration2Params) (extensions []Extension2, err error) {
	for _, ext := range params.Extensions {
		scount, err := ext.Sectors.Count()
		if err != nil {
			return nil, err
		}
		sectors, err := ext.Sectors.All(scount)
		if err != nil {
			return nil, err
		}
		sc, err := json.Marshal(ext.SectorsWithClaims)
		if err != nil {
			return nil, err
		}
		extensions = append(extensions, Extension2{
			Deadline:          ext.Deadline,
			Partition:         ext.Partition,
			Sectors:           ArrayToString(sectors),
			SectorsWithClaims: sc,
			NewExpiration:     ext.NewExpiration,
		})
	}
	return
}

type CID struct {
	cid.Cid
}

// Scan value into string, implements sql.Scanner interface
func (t *CID) Scan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("failed to scan value into CID")
	}
	result, err := cid.Decode(v)
	if err != nil {
		return fmt.Errorf("failed to decode CID: %w", err)
	}
	t.Cid = result
	return nil
}

// Value return json value, implement driver.Valuer interface
func (t CID) Value() (driver.Value, error) {
	return t.String(), nil
}

type Address struct {
	address.Address
}

// Scan value into string, implements sql.Scanner interface
func (t *Address) Scan(value interface{}) error {
	v, ok := value.(string)
	if !ok {
		return fmt.Errorf("failed to scan value into Address")
	}
	result, err := address.NewFromString(v)
	if err != nil {
		return fmt.Errorf("failed to decode Address: %w", err)
	}
	t.Address = result
	return nil
}

// Value return json value, implement driver.Valuer interface
func (t Address) Value() (driver.Value, error) {
	return t.String(), nil
}
