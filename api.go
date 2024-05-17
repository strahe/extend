package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

func NewRouter(srv *Service, secret []byte) http.Handler {
	r := mux.NewRouter()
	r.Use(authMiddleware(secret))

	impl := newImplAPI(srv)
	r.HandleFunc("/requests", impl.create).Methods("POST")
	r.HandleFunc("/requests/{id:[0-9]+}", impl.get).Methods("GET")
	return r
}

type implAPI struct {
	validate *validator.Validate
	srv      *Service
}

func newImplAPI(srv *Service) *implAPI {
	return &implAPI{
		validator.New(validator.WithRequiredStructEnabled()),
		srv,
	}
}

type createRequestArgs struct {
	Miner         address.Address `json:"miner"`          // miner address
	From          time.Time       `json:"from"`           // expiration from
	To            time.Time       `json:"to"`             //  expiration to
	Extension     *abi.ChainEpoch `json:"extension"`      // extension to set
	NewExpiration *abi.ChainEpoch `json:"new_expiration"` // new expiration to set
	Tolerance     *abi.ChainEpoch `json:"tolerance"`      // tolerance for expiration
	MaxSectors    *int            `json:"max_sectors"`    // max sectors to include in a single message
	DryRun        bool            `json:"dry_run"`
}

func (a *implAPI) create(w http.ResponseWriter, r *http.Request) {
	var args createRequestArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		warpResponse(w, http.StatusBadRequest, nil, err)
		return
	}
	if err := a.validate.Struct(args); err != nil {
		warpResponse(w, http.StatusBadRequest, nil, err)
		return
	}
	if args.Miner.Empty() {
		warpResponse(w, http.StatusBadRequest, nil, fmt.Errorf("miner address is empty"))
		return
	}
	if args.Extension == nil && args.NewExpiration == nil {
		warpResponse(w, http.StatusBadRequest, nil, fmt.Errorf("either extension or new_expiration must be set"))
		return
	}
	fromEpoch := TimestampToEpoch(args.From)
	toEpoch := TimestampToEpoch(args.To)
	if toEpoch < fromEpoch {
		warpResponse(w, http.StatusBadRequest, nil, fmt.Errorf("to must be greater than from"))
		return
	}

	var maxSectors int
	if args.MaxSectors == nil {
		maxSectors = 500 // default value
	} else {
		maxSectors = *args.MaxSectors
	}
	if maxSectors < 0 {
		warpResponse(w, http.StatusBadRequest, nil, fmt.Errorf("max_sectors must be greater than 0"))
		return
	}

	req, err := a.srv.createRequest(r.Context(), args.Miner, args.From, args.To,
		args.Extension, args.NewExpiration, args.Tolerance, maxSectors, args.DryRun)
	if err != nil {
		warpResponse(w, http.StatusBadRequest, nil, err)
		return
	}
	if req.Messages == nil {
		req.Messages = make([]*Message, 0)
	}
	warpResponse(w, http.StatusOK, req, nil)
}

func (a *implAPI) get(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		warpResponse(w, http.StatusBadRequest, nil, fmt.Errorf("invalid id: %s", vars["id"]))
		return
	}

	req, err := a.srv.getRequest(r.Context(), uint(id))
	if err != nil {
		warpResponse(w, http.StatusBadRequest, nil, err)
		return
	}
	warpResponse(w, http.StatusOK, req, nil)
}

type response struct {
	Data  interface{} `json:"data,omitempty"`
	Error *string     `json:"error,omitempty"`
}

func warpResponse(w http.ResponseWriter, code int, data interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	resp := response{Data: data}
	if err != nil {
		msg := err.Error()
		resp.Error = &msg
	}
	payload, err := json.Marshal(resp) // nolint: errcheck
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(payload) // nolint: errcheck
}
