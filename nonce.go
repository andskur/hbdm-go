package hbdm

import (
	"errors"
	"io/ioutil"
	"os"
	"strconv"
)

const (
	nonceFile = "data/nonce"
)

var (
	errREadNonceFile = errors.New("nonce file read error")
)

func (h *Hbdm) GetAndIncrementNonce() (nonce uint64, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	nonce, err = readNonce()
	if err != nil {
		return
	}
	err = incrementNonce(&nonce)
	return
}

func readNonce() (nonce uint64, err error) {
	if err = CreateNonceFileIfNotExists(); err != nil {
		return
	}
	data, err := ioutil.ReadFile(nonceFile)
	if err != nil {
		return 0, errREadNonceFile
	}
	nonce, err = strconv.ParseUint(string(data), 10, 64)
	return
}
func WriteNonce(data []byte) (err error) {
	err = ioutil.WriteFile(nonceFile, data, 0644)
	return
}

func incrementNonce(nonceOld *uint64) (err error) {
	*nonceOld = *nonceOld + 1
	ns := strconv.FormatUint(*nonceOld, 10)
	err = WriteNonce([]byte(ns))
	return
}

func CreateNonceFileIfNotExists() (err error) {
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		if err := os.Mkdir("data", 0700); err != nil {
			return err
		}
	}
	if _, err = os.Stat(nonceFile); os.IsNotExist(err) {
		if _, err = os.Create(nonceFile); err != nil {
			return err
		}
		d1 := []byte("1")
		err = WriteNonce(d1)
		return
	}
	return err
}
