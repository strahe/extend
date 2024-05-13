package main

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/lotus/chain/actors/builtin/miner"
	"github.com/ipfs/go-cid"
	"gorm.io/gorm"
)

type RequestStatus string

const (
	RequestStatusCreated RequestStatus = "created"
	RequestStatusPending               = "pending"
	RequestStatusFailed                = "failed"
	RequestStatusSuccess               = "success"
)

// Request represents a request in the system.
type Request struct {
	ID            uint            `json:"id" gorm:"primarykey"`
	Miner         Address         `gorm:"index" json:"miner"`
	From          time.Time       `json:"from"`
	To            time.Time       `json:"to"`
	Extension     *abi.ChainEpoch `json:"extension"`
	NewExpiration *abi.ChainEpoch `json:"new_expiration"`
	Status        RequestStatus   `gorm:"index" json:"status"`
	Messages      []*Message      `json:"messages"`
	Error         string          `json:"error"`
	Took          time.Duration   `json:"took"`
	ConfirmedAt   *time.Time      `json:"confirmed_at"`
	DryRun        bool            `json:"dry_run"`
	DryRunResult  string          `json:"dry_run_result"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	DeletedAt     gorm.DeletedAt  `gorm:"index" json:"-"`
}

// Message represents a message in the system.
// It contains a unique identifier (MsgCid) and a list of Extensions.
type Message struct {
	ID         uint           `gorm:"primarykey" json:"id"`
	Cid        CID            `gorm:"index" json:"cid"` // The unique identifier of the message
	Extensions []Extension2   `json:"extensions"`       // The list of extensions associated with the message
	RequestID  uint           `json:"request_id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
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
