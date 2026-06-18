package xrayapi

import (
	"fmt"

	commonserial "github.com/NlightN22/xray-p2p/go/internal/xrayapi/proto/gen/commonserial"
	"google.golang.org/protobuf/proto"
)

func typedMessage(msg proto.Message) (*commonserial.TypedMessage, error) {
	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal %s: %w", msg.ProtoReflect().Descriptor().FullName(), err)
	}
	return &commonserial.TypedMessage{
		Type:  string(msg.ProtoReflect().Descriptor().FullName()),
		Value: data,
	}, nil
}
