package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/goburrow/modbus"
	"github.com/google/logger"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DiscreteInput        = "discrete_input"
	Coil                 = "coil"
	InputRegister        = "input_register"
	InputRegisterFloat32 = "input_register_float"
	InputRegisterFloat64 = "input_register_double"
	HoldingRegister      = "holding_register"
	MaxRegisterQuantity  = 125
	CoilResetValue       = uint16(0)
)

var ipMutexMap = make(map[string]*sync.Mutex)
var mapMutex = &sync.Mutex{}

type Client interface {
	Close()
	Read(request ReadRequest) (ReadResponse, []error)
	Write(writeRequests []WriteRequest) error
}

type DefaultModbusClient struct {
	fullAddress string
	slaveId     int
	client      modbus.Client
	handle      modbus.ClientHandler
	commMutex   *sync.Mutex
}

// Pole adress pro čtení z cílového modubus device
type ReadRequest struct {
	DiscreteInputs        []int
	Coils                 []int
	InputRegisters        []int
	InputRegistersFloat32 []int
	InputRegistersFloat64 []int
	HoldingRegisters      []int
}

// Odpověď s mapami adresa -> hodnota pro každý typ objektu
type ReadResponse struct {
	DiscreteInputs        map[int]uint16
	Coils                 map[int]uint16
	InputRegisters        map[int]int16
	InputRegistersFloat32 map[int]float32
	InputRegistersFloat64 map[int]float64
	HoldingRegisters      map[int]int16
}

type WriteResponse struct {
	Coils                  map[int]uint16
	HoldingRegisters       map[int]uint16
	CoilsErrors            map[int]error
	HoldingRegistersErrors map[int]error
}

type WriteRequest interface {
	GetType() string
	GetAddress() int
	GetValue() uint16
	PreReset() bool
	OnSuccessWrite(value uint16)
	OnWriteError(err error)
}

type DefaultWriteRequest struct {
	Type    string
	Address int
	Value   uint16
	Reset   bool
}

func (dwr *DefaultWriteRequest) GetType() string {
	return dwr.Type
}

func (dwr *DefaultWriteRequest) GetAddress() int {
	return dwr.Address
}

func (dwr *DefaultWriteRequest) GetValue() uint16 {
	return dwr.Value
}

func (dwr *DefaultWriteRequest) PreReset() bool {
	return dwr.Reset
}

func (dwr *DefaultWriteRequest) OnSuccessWrite(value uint16) {

}

func (dwr *DefaultWriteRequest) OnWriteError(err error) {

}

func getMutexForIP(ip string) *sync.Mutex {
	mapMutex.Lock()
	defer mapMutex.Unlock()
	mutex, ok := ipMutexMap[ip]
	if !ok {
		mutex = &sync.Mutex{}
		ipMutexMap[ip] = mutex
	}
	return mutex
}

func CreateModbusClient(url string, port int, slaveId int) (Client, error) {
	fullAddress := fmt.Sprintf("%s:%d", url, port)
	handler := modbus.NewTCPClientHandler(fullAddress)
	handler.Timeout = 10 * time.Second
	handler.SlaveId = byte(slaveId)
	//handler.Logger = log.New(os.Stdout, fmt.Sprintf("modbus [%s(%d)]:", fullAddress, slaveId), log.LstdFlags)
	mutex := getMutexForIP(url)
	mutex.Lock()
	err := handler.Connect()
	if err != nil {
		mutex.Unlock()
		_ = handler.Close()
		return nil, err
	}
	client := modbus.NewClient(handler)
	return DefaultModbusClient{
		client:      client,
		handle:      handler,
		fullAddress: fullAddress,
		slaveId:     slaveId,
		commMutex:   mutex,
	}, nil
}

func (dmb DefaultModbusClient) Close() {
	defer dmb.commMutex.Unlock()
	tcpHandler, ok := dmb.handle.(*modbus.TCPClientHandler)
	if ok {
		err := tcpHandler.Close()
		if err != nil {
			logger.Errorf("chyba pri uzavirani tcp handleru pro %s: %s", dmb.String(), err.Error())
		}
	}
}

func (dmb DefaultModbusClient) Read(request ReadRequest) (ReadResponse, []error) {
	response := CreateEmptyReadResponse()
	errorsSlice := make([]error, 0)
	if len(request.HoldingRegisters) != 0 {
		for _, ranges := range CalculateAddressRangeMulti(request.HoldingRegisters) {
			holdingRegisters, err := dmb.ReadMultipleHoldingRegisters(ranges[0], ranges[1])
			if err != nil {
				logger.Errorf("%s chyba pri hromadnem cteni registru: %s", dmb.String(), err.Error())
				errorsSlice = append(errorsSlice, err)
			}
			for k, v := range holdingRegisters {
				response.HoldingRegisters[k] = v
			}
		}
	}
	if len(request.Coils) != 0 {
		coils, err := dmb.ReadMultipleCoils(CalculateAddressRange(request.Coils))
		if err != nil {
			logger.Errorf("%s chyba pro hromadnem cteni civek: %s", dmb.String(), err.Error())
			errorsSlice = append(errorsSlice, err)
		}
		response.Coils = coils
	}
	if len(request.DiscreteInputs) != 0 {
		inputs, err := dmb.ReadMultipleDiscreteInputs(CalculateAddressRange(request.DiscreteInputs))
		if err != nil {
			logger.Errorf("%s chyba pro hromadnem cteni diskretnich vstupu: %s", dmb.String(), err.Error())
			errorsSlice = append(errorsSlice, err)
		}
		response.DiscreteInputs = inputs
	}
	if len(request.InputRegisters) != 0 {
		inputs, err := dmb.ReadMultipleInputRegisters(CalculateAddressRange(request.InputRegisters))
		if err != nil {
			logger.Errorf("%s chyba pro hromadnem cteni vstupich registru: %s", dmb.String(), err.Error())
			errorsSlice = append(errorsSlice, err)
		}
		response.InputRegisters = inputs
	}
	if len(request.InputRegistersFloat32) != 0 {
		inputs, err := dmb.ReadMultipleInputRegistersFloat32(CalculateAddressRange(request.InputRegistersFloat32))
		if err != nil {
			logger.Errorf("%s chyba pro hromadnem cteni vstupich registru (float32): %s", dmb.String(), err.Error())
			errorsSlice = append(errorsSlice, err)
		}
		response.InputRegistersFloat32 = inputs
	}
	if len(request.InputRegistersFloat64) != 0 {
		inputs, err := dmb.ReadMultipleInputRegistersFloat64(CalculateAddressRange(request.InputRegistersFloat64))
		if err != nil {
			logger.Errorf("%s chyba pro hromadnem cteni vstupich registru (float64): %s", dmb.String(), err.Error())
			errorsSlice = append(errorsSlice, err)
		}
		response.InputRegistersFloat64 = inputs
	}
	return response, errorsSlice
}

func (dmb DefaultModbusClient) ReadMultipleHoldingRegisters(startAddress, lastAddress int) (map[int]int16, error) {
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa registru je nizsi ne prvni adresa registru")
	}
	//logger.Infof("%s - cteni holdnig registru od: %d - do: %d", dmb.String(), startAddress, lastAddress)
	registerMap := make(map[int]int16)

	var realStart, realEnd int
	loops := (lastAddress-startAddress+1)/MaxRegisterQuantity + 1

	realStart = startAddress
	if lastAddress-realStart+1 > MaxRegisterQuantity {
		realEnd = startAddress + MaxRegisterQuantity - 1
	} else {
		realEnd = lastAddress
	}

	for i := 0; i < loops; i++ {
		result, err := dmb.client.ReadHoldingRegisters(uint16(realStart), uint16(realEnd-realStart+1))
		if err != nil {
			return map[int]int16{}, err
		}
		for j := 0; j < len(result); j = j + 2 {
			registerMap[j/2+realStart] = int16(binary.BigEndian.Uint16(result[j : j+2]))
		}

		realStart = realStart + MaxRegisterQuantity + 1
		realEnd = int(math.Min(float64(realEnd+MaxRegisterQuantity), float64(lastAddress))) + 1
	}

	return registerMap, nil
}

func (dmb DefaultModbusClient) ReadMultipleCoils(startAddress, lastAddress int) (map[int]uint16, error) {
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa cívky je nizsi ne prvni adresa registru")
	}
	//logger.Infof("%s - cteni civek od: %d - do: %d", dmb.String(), startAddress, lastAddress)
	coilsMap := make(map[int]uint16)
	result, err := dmb.client.ReadCoils(uint16(startAddress), uint16(lastAddress-startAddress+1))
	if err != nil {
		return map[int]uint16{}, err
	}
	binaryString := byteSliceToBinaryString(result)
	for i, coil := range strings.Split(binaryString, "") {
		bitVal, _ := strconv.ParseUint(coil, 2, 1)
		coilsMap[i+startAddress] = uint16(bitVal)
	}
	return coilsMap, err
}

func (dmb DefaultModbusClient) ReadMultipleDiscreteInputs(startAddress, lastAddress int) (map[int]uint16, error) {
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa cívky je nizsi ne prvni adresa registru")
	}
	inputsMap := make(map[int]uint16)
	result, err := dmb.client.ReadDiscreteInputs(uint16(startAddress), uint16(lastAddress-startAddress+1))
	if err != nil {
		return map[int]uint16{}, err
	}
	binaryString := byteSliceToBinaryString(result)
	for i, coil := range strings.Split(binaryString, "") {
		bitVal, _ := strconv.ParseUint(coil, 2, 1)
		inputsMap[i+startAddress] = uint16(bitVal)
	}
	return inputsMap, err
}

func (dmb DefaultModbusClient) ReadMultipleInputRegisters(startAddress, lastAddress int) (map[int]int16, error) {
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa registru je nizsi ne prvni adresa registru")
	}
	//logger.Infof("%s - cteni holdnig registru od: %d - do: %d", dmb.String(), startAddress, lastAddress)
	registerMap := make(map[int]int16)

	var realStart, realEnd int
	loops := (lastAddress-startAddress+1)/MaxRegisterQuantity + 1

	realStart = startAddress
	if lastAddress-realStart+1 > MaxRegisterQuantity {
		realEnd = startAddress + MaxRegisterQuantity - 1
	} else {
		realEnd = lastAddress
	}

	for i := 0; i < loops; i++ {
		result, err := dmb.client.ReadInputRegisters(uint16(realStart), uint16(realEnd-realStart+1))
		if err != nil {
			return map[int]int16{}, err
		}
		for j := 0; j < len(result); j = j + 2 {
			registerMap[j/2+realStart] = int16(binary.BigEndian.Uint16(result[j : j+2]))
		}

		realStart = realStart + MaxRegisterQuantity + 1
		realEnd = int(math.Min(float64(realEnd+MaxRegisterQuantity), float64(lastAddress))) + 1
	}

	return registerMap, nil
}

func (dmb DefaultModbusClient) ReadMultipleInputRegistersFloat32(startAddress, lastAddress int) (map[int]float32, error) {
	lastAddress++
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa registru je nizsi ne prvni adresa registru")
	}
	//logger.Infof("%s - cteni holdnig registru od: %d - do: %d", dmb.String(), startAddress, lastAddress)
	registerMap := make(map[int]float32)

	var realStart, realEnd int
	loops := (lastAddress-startAddress+1)/MaxRegisterQuantity + 1

	realStart = startAddress
	if lastAddress-realStart+1 > MaxRegisterQuantity {
		realEnd = startAddress + MaxRegisterQuantity - 1
	} else {
		realEnd = lastAddress
	}

	for i := 0; i < loops; i++ {
		result, err := dmb.client.ReadInputRegisters(uint16(realStart), uint16(realEnd-realStart+1))
		if err != nil {
			return map[int]float32{}, err
		}
		for j := 0; j < len(result); j = j + 2 {
			registerMap[j/2+realStart] = math.Float32frombits(binary.BigEndian.Uint32(result[j : j+4]))

		}

		realStart = realStart + MaxRegisterQuantity + 1
		realEnd = int(math.Min(float64(realEnd+MaxRegisterQuantity), float64(lastAddress))) + 1
	}

	return registerMap, nil
}

func (dmb DefaultModbusClient) ReadMultipleInputRegistersFloat64(startAddress, lastAddress int) (map[int]float64, error) {
	lastAddress++
	if lastAddress < startAddress {
		return nil, errors.New("posledni adresa registru je nizsi ne prvni adresa registru")
	}
	registerMap := make(map[int]float64)

	var realStart, realEnd int
	loops := (lastAddress-startAddress+1)/MaxRegisterQuantity + 1

	realStart = startAddress
	if lastAddress-realStart+1 > MaxRegisterQuantity {
		realEnd = startAddress + MaxRegisterQuantity - 1
	} else {
		realEnd = lastAddress
	}

	for i := 0; i < loops; i++ {
		result, err := dmb.client.ReadInputRegisters(uint16(realStart), uint16(realEnd-realStart+1))
		if err != nil {
			return map[int]float64{}, err
		}
		for j := 0; j < len(result); j = j + 4 {
			registerMap[j/4+realStart] = math.Float64frombits(binary.BigEndian.Uint64(result[j : j+8]))
		}

		realStart = realStart + MaxRegisterQuantity + 1
		realEnd = int(math.Min(float64(realEnd+MaxRegisterQuantity), float64(lastAddress))) + 1
	}

	return registerMap, nil
}

func (dmb DefaultModbusClient) String() string {
	return fmt.Sprintf("[%s (slaveId: %d)]", dmb.fullAddress, dmb.slaveId)
}

func getCoilRealValue(val uint16) uint16 {
	if val == 1 {
		return 0xFF00
	} else {
		return 0x0000
	}
}

func (dmb DefaultModbusClient) Write(requests []WriteRequest) error {
	for _, request := range requests {
		var result []byte
		var err error
		switch request.GetType() {
		case Coil:
			if request.PreReset() {
				result, err = dmb.client.WriteSingleCoil(uint16(request.GetAddress()), CoilResetValue)
				if err != nil {
					logger.Errorf("chyba pri zapisu civky: %s [%s]", err.Error(), dmb.String())
					request.OnWriteError(err)
					return err
				}
			}
			result, err = dmb.client.WriteSingleCoil(uint16(request.GetAddress()), getCoilRealValue(request.GetValue()))
			if err != nil {
				logger.Errorf("chyba pri zapisu civky: %s [%s]", err.Error(), dmb.String())
				request.OnWriteError(err)
			} else {
				request.OnSuccessWrite(binary.BigEndian.Uint16(result))
			}
		case HoldingRegister:
			result, err = dmb.client.WriteSingleRegister(uint16(request.GetAddress()), request.GetValue())
			if err != nil {
				request.OnWriteError(err)
			} else {
				request.OnSuccessWrite(binary.BigEndian.Uint16(result))
			}
		default:
			return errors.New(fmt.Sprintf("nepodporovany typ modbus operace: %s", request.GetType()))
		}

	}
	return nil
}
