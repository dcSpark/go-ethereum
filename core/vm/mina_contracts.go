package vm

// Solidity interfaces for precompiles
//
// enum NetworkId {
//     MAINNET,
//     TESTNET,
//     NULLNET
// }
//
// interface IHasher {
//     function poseidonHash(
//         NetworkId networkId,
//         bytes32[] memory fields
//     ) external view returns (bytes32);
// }
//
// interface ISigner {
//     function verify(
//         NetworkId networkId,
//         bytes32 pubKeyX,
//         bytes32 pubKeyY,
//         bytes32 signatureRX,
//         bytes32 signatureS,
//         bytes32[] calldata fields
//     ) external view returns (bool);
// }

/*
#cgo LDFLAGS: ${SRCDIR}/../../mina/lib/libmina.a -ldl
#include "../../mina/lib/mina.h"
*/
import "C"
import (
	"bytes"
	"errors"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

var sol_bool, _ = abi.NewType("bool", "", nil)
var sol_uint8, _ = abi.NewType("uint8", "", nil)
var sol_string, _ = abi.NewType("string", "", nil)
var sol_bytes32, _ = abi.NewType("bytes32", "", nil)
var sol_bytes32Arr, _ = abi.NewType("bytes32[]", "", nil)

var revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]

func packErr(message string) []byte {
	bytes, err := abi.Arguments{{Type: sol_string}}.Pack(message)

	if err != nil {
		return nil
	}

	return append(revertSelector, bytes...)
}

type MinaPoseidon struct{}

func (c *MinaPoseidon) RequiredGas(input []byte) uint64 {
	return 1000
}

// 0x1f831f84
var poseidonHashSignature = crypto.Keccak256([]byte("poseidonHash(uint8,bytes32[])"))[:4]

var (
	errMinaPoseidonInvalidSignature = errors.New("invalid function signature")
	errMinaPoseidonCallingRustLibFailed = errors.New("calling rust library failed")
)

func (c *MinaPoseidon) Run(input []byte) ([]byte, error) {
	if len(input) < 4 || !bytes.Equal(input[:4], poseidonHashSignature) {
		return packErr("Invalid signature"), errMinaPoseidonInvalidSignature
	}

	calldata := input[4:]

	unpacked, err := (abi.Arguments{{
		Type: sol_uint8}, // networkId
		{Type: sol_bytes32Arr}, // fields
	}).Unpack(calldata)

	if err != nil {
		return packErr("Unable to unpack calldata"), err
	}

	networkId := unpacked[0].(uint8)
	fields := unpacked[1].([][32]uint8)

	output_buffer := [32]byte{}

	var fields_ptr *C.uint8_t
	if len(fields) == 0 {
		fields_ptr = (*C.uint8_t)(nil)
	} else {
		fields_ptr = (*C.uint8_t)(&fields[0][0])
	}

	if !C.poseidon(
		C.uint8_t(networkId),
		fields_ptr,
		C.uintptr_t(len(fields)),
		(*C.uint8_t)(&output_buffer[0]),
	) {
		return packErr("Calling Poseidon hash failed"), errMinaPoseidonCallingRustLibFailed
	}

	return output_buffer[:], nil
}

type MinaSigner struct{}

func (c *MinaSigner) RequiredGas(input []byte) uint64 {
	return 1000
}

// 0x462e39d6
var verifySignature = crypto.Keccak256([]byte("verify(uint8,bytes32,bytes32,bytes32,bytes32,bytes32[])"))[:4]

func (c *MinaSigner) Run(input []byte) ([]byte, error) {
	if len(input) < 4 || !bytes.Equal(input[:4], verifySignature) {
		return packErr("Invalid signature"), ErrExecutionReverted
	}

	calldata := input[4:]

	unpacked, err := (abi.Arguments{
		{Type: sol_uint8},      // networkId
		{Type: sol_bytes32},    // pubKeyX
		{Type: sol_bytes32},    // pubKeyY
		{Type: sol_bytes32},    // signatureRX
		{Type: sol_bytes32},    // signatureS
		{Type: sol_bytes32Arr}, // fields
	}).Unpack(calldata)

	if err != nil {
		return packErr("Unable to unpack calldata"), err
	}

	networkId := unpacked[0].(uint8)
	pubKeyX := unpacked[1].([32]uint8)
	pubKeyY := unpacked[2].([32]uint8)
	signatureRX := unpacked[3].([32]uint8)
	signatureS := unpacked[4].([32]uint8)
	fields := unpacked[5].([][32]uint8)

	if len(fields) == 0 {
		return packErr("Unable to verify for 0 fields"), ErrExecutionReverted
	}

	output_buffer := false

	if !C.verify(
		C.uint8_t(networkId),
		(*C.uint8_t)(&pubKeyX[0]),
		(*C.uint8_t)(&pubKeyY[0]),
		(*C.uint8_t)(&signatureRX[0]),
		(*C.uint8_t)(&signatureS[0]),
		(*C.uint8_t)(&fields[0][0]),
		C.uintptr_t(len(fields)),
		(*C.bool)(&output_buffer),
	) {
		return packErr("Calling verify failed"), ErrExecutionReverted
	}

	return abi.Arguments{{Type: sol_bool}}.Pack(output_buffer)
}
