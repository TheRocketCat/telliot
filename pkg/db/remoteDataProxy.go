// Copyright (c) The Tellor Authors.
// Licensed under the MIT License.

package db

import (
	"crypto/ecdsa"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	lru "github.com/hashicorp/golang-lru"
	"github.com/pkg/errors"
	"github.com/tellor-io/telliot/pkg/config"
	"github.com/tellor-io/telliot/pkg/util"
)

// how long a signed request is good for before reject it. Semi-protection against replays.
const _validityThreshold = 2 //seconds

/***************************************************************************************
** NOTE: This component is used to proxy data requests from approved miner processes. Miner
** public addresses are whitelisted and a small history of requests is retained to mitigate
** replay attacks. All incoming requests must be signed by the miner making the request so
** that the miner's public address can be verified. Best practice is to batch data lookups
** into single requests to improve performance and security.
**
** This component does NOT, repeat NOT, prevent DDoS attacks. Users must
** use their own solution to prevent such attacks if operating this code in a publicly
** accessible environment. USE AT YOUR OWN RISK
***************************************************************************************/

type remoteImpl struct {
	privateKey    *ecdsa.PrivateKey
	publicAddress string
	localDB       DB
	whitelist     map[string]bool
	postURL       string
	logger        log.Logger
	wlHistory     map[string]*lru.ARCCache
	rwLock        sync.RWMutex
}

// OpenRemoteDB establishes a proxy to a remote data server.
func OpenRemoteDB(cfg *config.Config, localDB DB) (DataServerProxy, error) {
	logger := log.With(util.NewLogger(), "db", "remoteDb")

	privateKey, err := crypto.HexToECDSA(os.Getenv(config.PrivateKeyEnvName))
	if err != nil {
		level.Error(logger).Log("msg", "problem decoding private key", "err", err)
		return nil, err
	}
	//get address from config
	_fromAddress := cfg.PublicAddress

	//convert to address
	fromAddress := common.HexToAddress(_fromAddress)

	whitelist := cfg.ServerWhitelist
	wlMap := make(map[string]bool)
	wlLRU := make(map[string]*lru.ARCCache)
	for _, a := range whitelist {
		addr := common.HexToAddress(a)
		asStr := strings.ToLower(addr.Hex())
		hist, err := lru.NewARC(50)
		if err != nil {
			return nil, err
		}
		wlLRU[asStr] = hist
		wlMap[asStr] = true
	}

	url := "http://" + cfg.Mine.RemoteDBHost + ":" + strconv.Itoa(int(cfg.Mine.RemoteDBPort))
	i := &remoteImpl{
		privateKey:    privateKey,
		publicAddress: strings.ToLower(fromAddress.Hex()),
		localDB:       localDB,
		postURL:       url,
		whitelist:     wlMap,
		wlHistory:     wlLRU,
		logger:        logger,
	}

	level.Info(i.logger).Log(
		"msg", "created remote data proxy connector",
		"host", cfg.Mine.RemoteDBHost,
		"port", cfg.Mine.RemoteDBPort,
	)

	return i, nil
}

// Check whether an incoming storage request key is prefixed by one of our known miner
// keys. Otherwise, we have to reject the request as invalid or not coming from this
// codebase.
func (i *remoteImpl) hasAddressPrefix(key string) bool {
	for k := range i.whitelist {
		if strings.HasPrefix(key, k) {
			return true
		}
	}
	return false
}

func (i *remoteImpl) IncomingRequest(data []byte) ([]byte, error) {
	req, err := decodeRequest(data, i)
	if err != nil {
		level.Error(i.logger).Log("msg", "problem decoding incoming request", "err", err)
		return errorResponse(err.Error())
	}

	if req == nil {
		return errorResponse("Could not decode request!")
	}

	if req.dbKeys == nil {
		return errorResponse("No keys found in request!")
	}

	if i.localDB == nil {
		return errorResponse("Missing localDB instance!")
	}

	if req.dbValues != nil && len(req.dbValues) > 0 {

		//lock out other threads from reading/writing until the write is done
		i.rwLock.Lock()

		//make sure we unlock when we leave so next thread can get in
		defer i.rwLock.Unlock()

		//if we're writing values to the DB
		if len(req.dbKeys) != len(req.dbValues) {
			return errorResponse("Keys and values must have the same array dimensions")
		}

		//request to write data locally
		for idx, k := range req.dbKeys {
			//make sure key is prefixed with address
			if !i.hasAddressPrefix(k) {
				return errorResponse("All remote data storage request keys must be prefixed with miner public Ethereum address")
			}
			v := req.dbValues[idx]
			if err := i.localDB.Put(k, v); err != nil {
				return errorResponse(err.Error())
			}
		}

	} else {
		//we're not writing, so we just need a read lock
		i.rwLock.RLock()
		defer i.rwLock.RUnlock()
	}

	level.Info(i.logger).Log("msg", "getting remote request for keys", "dbKeys", req.dbKeys)

	outMap := map[string][]byte{}
	for _, k := range req.dbKeys {
		if req.dbValues == nil && !isKnownKey(k) {
			return errorResponse("Invalid lookup key: " + k)
		}
		level.Debug(i.logger).Log("msg", "looking up local DB key", "key", k)
		bts, err := i.localDB.Get(k)

		if err != nil {
			return errorResponse(err.Error())
		}
		if bts != nil {
			level.Debug(i.logger).Log("msg", "get result byte size", "bytes", len(bts))
			outMap[k] = bts
		}
	}

	resp := &responsePayload{dbVals: outMap, errorMsg: ""}
	return encodeResponse(resp)
}

func (i *remoteImpl) Get(key string) ([]byte, error) {
	keys := []string{key}
	res, err := i.BatchGet(keys)
	if err != nil {
		return nil, err
	}
	return res[key], nil
}

func (i *remoteImpl) Put(key string, value []byte) (map[string][]byte, error) {
	//every key must be prefixed with miner's address

	keys := []string{key}
	vals := [][]byte{value}
	return i.BatchPut(keys, vals)
}

func (i *remoteImpl) BatchGet(keys []string) (map[string][]byte, error) {
	req, err := createRequest(keys, nil, i)
	if err != nil {
		return nil, err
	}
	data, err := encodeRequest(req)
	if err != nil {
		return nil, err
	}
	httpReq := &util.HTTPFetchRequest{Method: util.POST, QueryURL: i.postURL, Payload: data, Timeout: time.Duration(10 * time.Second)}

	respData, err := util.HTTPWithRetries(httpReq)
	if err != nil {
		return nil, errors.Wrapf(err, "retrieving data after retries")
	}
	remResp, err := decodeResponse(respData)
	if err != nil {
		return nil, err
	}
	if len(remResp.errorMsg) > 0 {
		return nil, errors.New(remResp.errorMsg)
	}
	return remResp.dbVals, nil
}

func (i *remoteImpl) BatchPut(keys []string, values [][]byte) (map[string][]byte, error) {
	//must prefix all keys with public address
	dbKeys := make([]string, len(keys))
	for idx, k := range keys {
		if !strings.HasPrefix(k, i.publicAddress) {
			dbKeys[idx] = i.publicAddress + "-" + k
		} else {
			dbKeys[idx] = k
		}
	}
	req, err := createRequest(dbKeys, values, i)
	if err != nil {
		return nil, err
	}
	data, err := encodeRequest(req)
	if err != nil {
		return nil, err
	}
	httpReq := &util.HTTPFetchRequest{
		Method:   util.POST,
		QueryURL: i.postURL,
		Payload:  data,
		Timeout:  time.Duration(10 * time.Second),
	}
	respData, err := util.HTTPWithRetries(httpReq)
	if err != nil {
		//return nil, err
		return nil, errors.Wrap(err, "put data after retries")
	}
	remResp, err := decodeResponse(respData)
	if err != nil {
		return nil, err
	}
	if len(remResp.errorMsg) > 0 {
		return nil, errors.New(remResp.errorMsg)
	}
	return remResp.dbVals, nil
}

func (i *remoteImpl) Sign(hash []byte) ([]byte, error) {
	return crypto.Sign(hash, i.privateKey)
}

func (i *remoteImpl) Verify(hash []byte, timestamp int64, sig []byte) error {
	pubKey, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return err
	}
	addr := crypto.PubkeyToAddress(*pubKey)
	ashex := strings.ToLower(addr.Hex())
	level.Debug(i.logger).Log(
		"msg", "verifying signature",
		"address", ashex,
		"whitelist", i.whitelist[ashex],
	)
	if !i.whitelist[ashex] {
		level.Warn(i.logger).Log(
			"msg", "unauthorized miner detected",
			"address", ashex,
		)
		return errors.Errorf("Unauthorized")
	}

	cache := i.wlHistory[ashex]
	if cache == nil {
		return errors.Errorf("No history found for address")
	}
	if cache.Contains(timestamp) {
		level.Debug(i.logger).Log(
			"msg", "miner already made request",
			"address", ashex,
			"timestamp", timestamp,
		)
		expr := time.Unix(timestamp+_validityThreshold, 0)
		now := time.Now()
		if now.After(expr) {
			level.Warn(i.logger).Log(
				"msg", "request time expired",
				"timestamp", time.Unix(timestamp, 0),
				"now", now,
			)
			return errors.Errorf("Request expired")
		}
		level.Debug(i.logger).Log(
			"msg", "time of last request",
			"comparing", expr,
			"to", now,
		)

	} else {
		level.Debug(i.logger).Log(
			"msg", "never seen miner before",
			"address", ashex,
			"timestamp", timestamp,
		)
	}
	cache.Add(timestamp, true)
	return nil
}
