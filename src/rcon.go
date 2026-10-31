package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type ServerResponse struct {
	Size         int32
	ID           int32
	ResponseType int32
	Response     string
}

const AUTH_FAILED = -1
const EMPTY_STRING_SIZE = 2
const INT32_SIZE = 4
const FIXED_ID = 1
const DEFAULT_BUFFER_SIZE = 4110 // largest packet the server sends: 12 byte header, 4096 byte body, 2 null bytes

func writeToServer(conn net.Conn, Type int32, command string) error {

	_, err := conn.Write((constructByteBuffer(FIXED_ID, Type, command)))
	if err != nil {
		return fmt.Errorf("-writeToServer: %w", err)
	}

	return nil
}
func constructByteBuffer(ID int32, commandType int32, command string) []byte {

	var buf bytes.Buffer                                                               // have an address, thus can call a pointer methods
	var size int32 = INT32_SIZE + INT32_SIZE + EMPTY_STRING_SIZE + int32(len(command)) //casts to int32

	binary.Write(&buf, binary.LittleEndian, size)
	binary.Write(&buf, binary.LittleEndian, ID)
	binary.Write(&buf, binary.LittleEndian, commandType)
	buf.WriteString(command)
	buf.Write([]byte{0, 0})

	return buf.Bytes()
}
func readFromServer(conn *net.TCPConn) (ServerResponse, error) {
	reply := make([]byte, DEFAULT_BUFFER_SIZE)

	_, err := conn.Read(reply)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-readFromServer: %w", err)
	}
	res, err := constructResponse(reply)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-readFromServer: %w", err)
	}

	return res, nil
}
func constructResponse(buf []byte) (ServerResponse, error) {
	reader := bytes.NewReader(buf)

	size, err := readInt32(reader)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-constructResponse: %w", err)
	}
	id, err := readInt32(reader)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-constructResponse: %w", err)
	}
	responseType, err := readInt32(reader)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-constructResponse: %w", err)
	}

	responseLen := size - INT32_SIZE - INT32_SIZE - EMPTY_STRING_SIZE
	response, err := readString(reader, responseLen)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-constructResponse: %w", err)
	}

	return ServerResponse{
		Size:         size,
		ID:           id,
		ResponseType: responseType,
		Response:     response,
	}, nil
}
func readInt32(buf io.Reader) (int32, error) {
	var val int32
	err := binary.Read(buf, binary.LittleEndian, &val)
	if err != nil {
		return 0, fmt.Errorf("-readInt32: %w", err)
	}

	return val, err
}
func readString(r io.Reader, length int32) (string, error) {
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return "", fmt.Errorf("-readString: %w", err)
	}

	return string(data), nil
}
func rconLogin(conn *net.TCPConn, rconPassword string) (ServerResponse, error) {
	commandType := int32(LOGIN)
	res, err := interact(conn, commandType, rconPassword)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-rconLogin: %w", err)
	}
	if res.ID == AUTH_FAILED {
		return res, fmt.Errorf("-rconLogin: failed to log in to RCON")
	}

	return res, err
}
func interact(conn *net.TCPConn, commandType int32, command string) (ServerResponse, error) {
	err := writeToServer(conn, commandType, command)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-interact: %w", err)
	}

	response, err := readFromServer(conn)
	if err != nil {
		return ServerResponse{}, fmt.Errorf("-interact: %w", err)
	}

	return response, nil
}
