//go:build rust

// The cgo bridge to the Rust bot engine. It is compiled only with the `rust`
// build tag so a default build needs no Rust toolchain.
//
// Build the library first, then the server:
//
//	cd backend/rust && cargo build --release -p tigerswap-bots
//	cd backend && go build -tags rust ./cmd/api
package botengine

/*
#cgo CFLAGS: -I${SRCDIR}
#cgo LDFLAGS: -L${SRCDIR}/../../rust/target/release -Wl,-rpath,${SRCDIR}/../../rust/target/release -ltigerswap_bots -ldl -lpthread -lm
#include <stdlib.h>

typedef struct BotEngine BotEngine;

BotEngine* bot_engine_new(void);
void bot_engine_free(BotEngine* engine);
unsigned long long bot_count(BotEngine* engine);
char* bot_upsert(BotEngine* engine, unsigned long long id, const char* strategy, const char* config);
char* bot_remove(BotEngine* engine, unsigned long long id);
char* bot_step(BotEngine* engine, unsigned long long id, const char* snapshot);
char* bot_arbitrage(BotEngine* engine, unsigned long long id, const char* venues);
char* bot_sniper(BotEngine* engine, unsigned long long id, const char* listing);
char* bot_rebalance(BotEngine* engine, unsigned long long id, const char* holdings);
char* bot_liquidation(BotEngine* engine, unsigned long long id, const char* position);
char* bot_copy(BotEngine* engine, unsigned long long id, const char* signals);
void bot_free(char* text);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"runtime"
	"unsafe"
)

type cgoEngine struct {
	handle *C.BotEngine
}

// Available reports whether the Rust engine is compiled into this binary.
func Available() bool {
	return true
}

// New builds the Rust engine. It returns an error only if the handle could not
// be created, which means allocation failed.
func New() (Engine, error) {
	handle := C.bot_engine_new()
	if handle == nil {
		return nil, errors.New("could not allocate the bot engine")
	}
	engine := &cgoEngine{handle: handle}
	runtime.SetFinalizer(engine, func(e *cgoEngine) { e.Close() })
	return engine, nil
}

// call invokes an FFI function, marshals the input, and returns the reply.
//
// The input and the returned string are both allocated in C and freed here, so
// no memory crosses the boundary unmanaged.
func (e *cgoEngine) call(input string, invoke func(handle *C.BotEngine, cInput *C.char) *C.char) (json.RawMessage, error) {
	if e.handle == nil {
		return nil, ErrUnavailable
	}
	cInput := C.CString(input)
	defer C.free(unsafe.Pointer(cInput))

	reply := invoke(e.handle, cInput)
	if reply == nil {
		return nil, errors.New("bot engine returned a null reply")
	}
	text := C.GoString(reply)
	C.bot_free(reply)
	return decodeEnvelope(text)
}

func (e *cgoEngine) Upsert(id uint64, strategy string, config json.RawMessage) error {
	if len(config) == 0 {
		config = json.RawMessage("{}")
	}
	cStrategy := C.CString(strategy)
	defer C.free(unsafe.Pointer(cStrategy))
	_, err := e.call(string(config), func(handle *C.BotEngine, cConfig *C.char) *C.char {
		return C.bot_upsert(handle, C.ulonglong(id), cStrategy, cConfig)
	})
	return err
}

func (e *cgoEngine) Remove(id uint64) error {
	_, err := e.call("{}", func(handle *C.BotEngine, _ *C.char) *C.char {
		return C.bot_remove(handle, C.ulonglong(id))
	})
	return err
}

func (e *cgoEngine) Step(id uint64, snapshot json.RawMessage) ([]Order, error) {
	data, err := e.call(string(snapshot), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_step(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	return decodeOrders(data)
}

func (e *cgoEngine) Arbitrage(id uint64, venues json.RawMessage) ([]Order, error) {
	data, err := e.call(string(venues), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_arbitrage(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	return decodeOrders(data)
}

func (e *cgoEngine) Sniper(id uint64, listing json.RawMessage) (*Order, error) {
	data, err := e.call(string(listing), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_sniper(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var order Order
	if err := json.Unmarshal(data, &order); err != nil {
		return nil, err
	}
	return &order, nil
}

func (e *cgoEngine) Rebalance(id uint64, holdings json.RawMessage) ([]RebalanceLeg, error) {
	data, err := e.call(string(holdings), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_rebalance(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	var legs []RebalanceLeg
	if err := json.Unmarshal(data, &legs); err != nil {
		return nil, err
	}
	return legs, nil
}

func (e *cgoEngine) Liquidation(id uint64, position json.RawMessage) (*Liquidation, error) {
	data, err := e.call(string(position), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_liquidation(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}
	var liquidation Liquidation
	if err := json.Unmarshal(data, &liquidation); err != nil {
		return nil, err
	}
	return &liquidation, nil
}

func (e *cgoEngine) Copy(id uint64, signals json.RawMessage) ([]Order, error) {
	data, err := e.call(string(signals), func(handle *C.BotEngine, cInput *C.char) *C.char {
		return C.bot_copy(handle, C.ulonglong(id), cInput)
	})
	if err != nil {
		return nil, err
	}
	return decodeOrders(data)
}

func (e *cgoEngine) Count() uint64 {
	if e.handle == nil {
		return 0
	}
	return uint64(C.bot_count(e.handle))
}

func (e *cgoEngine) Close() {
	if e.handle != nil {
		C.bot_engine_free(e.handle)
		e.handle = nil
		runtime.SetFinalizer(e, nil)
	}
}

func decodeOrders(data json.RawMessage) ([]Order, error) {
	var orders []Order
	if err := json.Unmarshal(data, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}
