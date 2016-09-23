package stream

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

const (
	SimpleString = '+'
	Error        = '-'
	Integer      = ':'
	BulkString   = '$'
	Array        = '*'
	Null         = '\x00'
)

var ErrInvalidSyntax = errors.New("invalid RESP syntax")

type RedisType struct {
	Type  byte
	Value interface{}
}

func (t RedisType) String() string {
	if v, ok := t.Value.(string); ok {
		return v
	}
	return ""
}

func (t RedisType) Integer() int64 {
	if v, ok := t.Value.(int64); ok {
		return v
	}
	return 0
}

func (t RedisType) Array() []RedisType {
	if v, ok := t.Value.([]RedisType); ok {
		return v
	}
	return nil
}

func (t RedisType) StringArray() []string {
	array := t.Array()
	out := make([]string, len(array))
	for i := range array {
		out[i] = array[i].String()
	}
	return out
}

func (t RedisType) Raw() []byte {
	out := []byte{t.Type}
	switch t.Type {
	case SimpleString, Error:
		out = append(out, []byte(t.String())...)
		out = append(out, '\r', '\n')
	case Integer:
		out = append(out, []byte(strconv.FormatInt(t.Integer(), 10))...)
		out = append(out, '\r', '\n')
	case BulkString:
		value := t.String()
		out = append(out, []byte(strconv.Itoa(len(value)))...)
		out = append(out, '\r', '\n')
		out = append(out, []byte(value)...)
		out = append(out, '\r', '\n')
	case Array:
		value := t.Array()
		out = append(out, []byte(strconv.Itoa(len(value)))...)
		out = append(out, '\r', '\n')
		for i := 0; i < len(value); i++ {
			out = append(out, value[i].Raw()...)
		}
	default:
		return nil
	}
	return out
}

func readLine(reader *bufio.Reader) ([]byte, error) {
	line, err := reader.ReadSlice('\n')
	if err != nil {
		return nil, err
	}

	if len(line) < 1 || line[len(line)-2] != '\r' {
		return nil, ErrInvalidSyntax
	}
	return line[:len(line)-2], nil
}

func ParseRESP(reader *bufio.Reader) (RedisType, error) {
	var out RedisType
	line, err := readLine(reader)
	if err != nil {
		return RedisType{}, err
	}

	if len(line) > 0 {
		out.Type = line[0]
		switch line[0] {
		case SimpleString, Error:
			out.Value = string(line[1:])
			return out, nil
		case Integer:
			out.Value, err = strconv.ParseInt(string(line[1:]), 10, 64)
			if err != nil {
				return RedisType{}, ErrInvalidSyntax
			}
			return out, nil
		case BulkString:
			length, err := strconv.Atoi(string(line[1:]))
			if err != nil {
				return RedisType{}, ErrInvalidSyntax
			}
			if length < 0 {
				return out, nil
			}
			buf := make([]byte, length+2)
			_, err = io.ReadFull(reader, buf)
			if err != nil {
				return RedisType{}, err
			}
			out.Value = string(buf[:length])
			return out, nil
		case Array:
			length, err := strconv.Atoi(string(line[1:]))
			if err != nil {
				return RedisType{}, ErrInvalidSyntax
			}
			if length < 0 {
				return out, nil
			}
			array := make([]RedisType, length)
			for i := 0; i < length; i++ {
				array[i], err = ParseRESP(reader)
				if err != nil {
					return RedisType{}, err
				}
			}
			out.Value = array
			return out, nil
		default:
			fmt.Println("Format not supported")
			return RedisType{}, ErrInvalidSyntax
		}
	} else {
		return RedisType{}, ErrInvalidSyntax
	}
}
