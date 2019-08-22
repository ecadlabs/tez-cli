package utils

import (
	"encoding/json"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Encoder interface {
	Encode(v interface{}) error
}

type NewEncoderFunc func(w io.Writer) Encoder

func GetEncoderFunc(format string) NewEncoderFunc {
	switch strings.ToLower(format) {
	case "json":
		return func(w io.Writer) Encoder {
			return json.NewEncoder(w)
		}

	case "yaml":
		return func(w io.Writer) Encoder {
			return yaml.NewEncoder(w)
		}
	}

	return nil
}
