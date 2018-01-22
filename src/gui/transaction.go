package gui

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/skycoin/skycoin/src/cipher"
	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/daemon"
	"github.com/skycoin/skycoin/src/visor"

	wh "github.com/skycoin/skycoin/src/util/http" //http,json helpers
)

// RegisterTxHandlers registers transaction handlers
func RegisterTxHandlers(mux *http.ServeMux, gateway *daemon.Gateway) {
	// get set of pending transactions
	mux.HandleFunc("/pendingTxs", getPendingTxs(gateway))
	// get latest confirmed transactions
	mux.HandleFunc("/lastTxs", getLastTxs(gateway))
	// get txn by txid
	mux.HandleFunc("/transaction", getTransactionByID(gateway))

	// Returns transactions that match the filters.
	// Method: GET
	// Args:
	//     addrs: Comma seperated addresses [optional, returns all transactions if no address is provided]
	//     confirmed: Whether the transactions should be confirmed [optional, must be 0 or 1; if not provided, returns all]
	mux.HandleFunc("/transactions", getTransactions(gateway))
	//inject a transaction into network
	mux.HandleFunc("/injectTransaction", injectTransaction(gateway))
	mux.HandleFunc("/resendUnconfirmedTxns", resendUnconfirmedTxns(gateway))
	// get raw tx by txid.
	mux.HandleFunc("/rawtx", getRawTx(gateway))
}

// Returns pending transactions
func getPendingTxs(gateway Gatewayer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}

		txns := gateway.GetAllUnconfirmedTxns()
		ret := make([]*visor.ReadableUnconfirmedTxn, 0, len(txns))
		for _, unconfirmedTxn := range txns {
			readable, err := visor.NewReadableUnconfirmedTxn(&unconfirmedTxn)
			if err != nil {
				logger.Error("%v", err)
				wh.Error500(w)
				return
			}
			ret = append(ret, readable)
		}

		wh.SendOr404(w, &ret)
	}
}

// DEPRECATED: last txs can't recover from db when restart
// , and it's not used actually
func getLastTxs(gateway *daemon.Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}
		txs, err := gateway.GetLastTxs()
		if err != nil {
			logger.Error("gateway.GetLastTxs failed: %v", err)
			wh.Error500(w)
			return
		}

		resTxs := make([]visor.TransactionResult, len(txs))
		for i, tx := range txs {
			rbTx, err := visor.NewReadableTransaction(tx)
			if err != nil {
				logger.Error("%v", err)
				wh.Error500(w)
				return
			}

			resTxs[i] = visor.TransactionResult{
				Transaction: *rbTx,
				Status:      tx.Status,
			}
		}

		wh.SendOr404(w, &resTxs)
	}
}

func getTransactionByID(gate Gatewayer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}
		txid := r.FormValue("txid")
		if txid == "" {
			wh.Error400(w, "txid is empty")
			return
		}

		h, err := cipher.SHA256FromHex(txid)
		if err != nil {
			wh.Error400(w, err.Error())
			return
		}

		tx, err := gate.GetTransaction(h)
		if err != nil {
			wh.Error400(w, err.Error())
			return
		}
		if tx == nil {
			wh.Error404(w)
			return
		}

		rbTx, err := visor.NewReadableTransaction(tx)
		if err != nil {
			logger.Error("%v", err)
			wh.Error500(w)
			return
		}

		resTx := visor.TransactionResult{
			Transaction: *rbTx,
			Status:      tx.Status,
		}
		wh.SendOr404(w, &resTx)
	}
}

// Returns transactions that match the filters.
// Method: GET
// URI: /transactions
// Args:
//     addrs: Comma seperated addresses [optional, returns all transactions if no address provided]
//     confirmed: Whether the transactions should be confirmed [optional, must be 0 or 1; if not provided, returns all]
func getTransactions(gateway *daemon.Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}

		// Gets 'addrs' parameter value
		addrs, err := parseAddressesFromStr(r.FormValue("addrs"))
		if err != nil {
			wh.Error400(w, fmt.Sprintf("parse parament: 'addrs' failed: %v", err))
			return
		}

		// Initialize transaction filters
		flts := []visor.TxFilter{visor.AddrsFilter(addrs)}

		// Gets the 'confirmed' parameter value
		confirmedStr := r.FormValue("confirmed")
		if confirmedStr != "" {
			confirmed, err := strconv.ParseBool(confirmedStr)
			if err != nil {
				wh.Error400(w, fmt.Sprintf("invalid 'confirmed' value: %v", err))
				return
			}

			flts = append(flts, visor.ConfirmedTxFilter(confirmed))
		}

		// Gets transactions
		txns, err := gateway.GetTransactions(flts...)
		if err != nil {
			logger.Error("get transactions failed: %v", err)
			wh.Error500(w)
			return
		}

		// Converts visor.Transaction to visor.TransactionResult
		txRlts, err := visor.NewTransactionResults(txns)
		if err != nil {
			logger.Error("Converts []visor.Transaction to visor.TransactionResults failed: %v", err)
			wh.Error500(w)
			return
		}

		wh.SendOr404(w, txRlts.Txns)
	}
}

// Parses comma seperated addresses string into []cipher.Address,
func parseAddressesFromStr(addrStr string) ([]cipher.Address, error) {
	if addrStr == "" {
		return nil, nil
	}

	var addrs []cipher.Address
	for _, as := range strings.Split(addrStr, ",") {
		s := strings.TrimSpace(as)
		if s == "" {
			continue
		}

		a, err := cipher.DecodeBase58Address(s)
		if err != nil {
			return nil, err
		}

		addrs = append(addrs, a)
	}

	return addrs, nil
}

//Implement
func injectTransaction(gateway Gatewayer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			wh.Error405(w)
			return
		}
		// get the rawtransaction
		v := struct {
			Rawtx string `json:"rawtx"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
			logger.Error("bad request: %v", err)
			wh.Error400(w, err.Error())
			return
		}

		b, err := hex.DecodeString(v.Rawtx)
		if err != nil {
			logger.Error("%v", err)
			wh.Error400(w, err.Error())
			return
		}

		txn, err := coin.TransactionDeserialize(b)
		if err != nil {
			logger.Error("%v", err)
			wh.Error400(w, err.Error())
			return
		}

		if err := gateway.InjectTransaction(txn); err != nil {
			wh.Error400(w, fmt.Sprintf("inject tx failed:%v", err))
			return
		}

		wh.SendOr404(w, txn.Hash().Hex())
	}
}

func resendUnconfirmedTxns(gate Gatewayer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}

		rlt := gate.ResendUnconfirmedTxns()
		v, _ := json.MarshalIndent(rlt, "", "    ")
		fmt.Println(v)
		wh.SendOr404(w, rlt)
		return
	}
}

func getRawTx(gate Gatewayer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			wh.Error405(w)
			return
		}
		txid := r.FormValue("txid")
		if txid == "" {
			wh.Error400(w, "txid is empty")
			return
		}

		h, err := cipher.SHA256FromHex(txid)
		if err != nil {
			wh.Error400(w, err.Error())
			return
		}

		tx, err := gate.GetTransaction(h)
		if err != nil {
			wh.Error400(w, err.Error())
			return
		}

		if tx == nil {
			wh.Error404(w)
			return
		}

		d := tx.Txn.Serialize()
		wh.SendOr404(w, hex.EncodeToString(d))
		return
	}
}
