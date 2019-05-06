package proxy

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"google.golang.org/grpc/encoding"
)

type jsonpbCodec struct {
	runtime.JSONPb
}

func (j *jsonpbCodec) Name() string {
	return "jsonpb"
}

// based on https://github.com/mwitkow/grpc-proxy
// Apache 2 License by Michal Witkowski (mwitkow)

// Codec returns a proxying encoding.Codec with the default protobuf codec as parent.
//
// See CodecWithParent.
func Codec() *rawCodec {
	return CodecWithParent(&jsonpbCodec{})
}

// CodecWithParent returns a proxying encoding.Codec with a user provided codec as parent.
func CodecWithParent(fallback encoding.Codec) *rawCodec {
	return &rawCodec{fallback}
}

type rawCodec struct {
	parentCodec encoding.Codec
}

type frame struct {
	payload []byte
}

func (c *rawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.payload, nil

}

func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.payload = data
	return nil
}

func (c *rawCodec) Name() string {
	return fmt.Sprintf("proxy>%s", c.parentCodec.Name())
}

func (c *rawCodec) String() string {
	return c.Name()
}

func (c *rawCodec) ContentType() string {
	return "application/json"
}

func (c *rawCodec) NewEncoder(w io.Writer) runtime.Encoder {
	f := func(v interface{}) error {
		data, err := c.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	return runtime.EncoderFunc(f)
}

func (c *rawCodec) NewDecoder(r io.Reader) runtime.Decoder {
	f := func(v interface{}) error {
		data, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		return c.Unmarshal(data, v)
	}
	return runtime.DecoderFunc(f)
}
