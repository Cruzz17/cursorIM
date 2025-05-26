package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// EncodingType 编码类型
type EncodingType string

const (
	EncodingJSON        EncodingType = "json"
	EncodingMessagePack EncodingType = "msgpack"
	EncodingProtobuf    EncodingType = "protobuf"
	EncodingCBOR        EncodingType = "cbor"
)

// MessageEncoder 消息编码器接口
type MessageEncoder interface {
	Encode(msg *Message) ([]byte, error)
	Decode(data []byte) (*Message, error)
	ContentType() string
	EncodingType() EncodingType
}

// JSONEncoder JSON编码器
type JSONEncoder struct{}

func NewJSONEncoder() *JSONEncoder {
	return &JSONEncoder{}
}

func (e *JSONEncoder) Encode(msg *Message) ([]byte, error) {
	return json.Marshal(msg)
}

func (e *JSONEncoder) Decode(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

func (e *JSONEncoder) ContentType() string {
	return "application/json"
}

func (e *JSONEncoder) EncodingType() EncodingType {
	return EncodingJSON
}

// TODO: MessagePackEncoder - 需要添加依赖 github.com/vmihailenco/msgpack/v5
// TODO: ProtobufEncoder - 需要添加依赖 google.golang.org/protobuf
// TODO: CBOREncoder - 需要添加依赖 github.com/fxamacker/cbor/v2

// EncoderFactory 编码器工厂
type EncoderFactory struct {
	encoders map[EncodingType]MessageEncoder
}

func NewEncoderFactory() *EncoderFactory {
	factory := &EncoderFactory{
		encoders: make(map[EncodingType]MessageEncoder),
	}

	// 注册默认编码器
	factory.RegisterEncoder(EncodingJSON, NewJSONEncoder())

	return factory
}

func (f *EncoderFactory) RegisterEncoder(encodingType EncodingType, encoder MessageEncoder) {
	f.encoders[encodingType] = encoder
}

func (f *EncoderFactory) GetEncoder(encodingType EncodingType) (MessageEncoder, error) {
	encoder, ok := f.encoders[encodingType]
	if !ok {
		return nil, fmt.Errorf("不支持的编码类型: %s", encodingType)
	}
	return encoder, nil
}

func (f *EncoderFactory) GetSupportedTypes() []EncodingType {
	types := make([]EncodingType, 0, len(f.encoders))
	for t := range f.encoders {
		types = append(types, t)
	}
	return types
}

// BenchmarkResult 性能测试结果
type BenchmarkResult struct {
	EncodingType     EncodingType  `json:"encoding_type"`
	EncodeTime       time.Duration `json:"encode_time"`       // 编码时间
	DecodeTime       time.Duration `json:"decode_time"`       // 解码时间
	EncodedSize      int           `json:"encoded_size"`      // 编码后大小
	CompressionRatio float64       `json:"compression_ratio"` // 压缩比
}

// BenchmarkEncoders 性能测试
func BenchmarkEncoders(msg *Message, iterations int) map[EncodingType]*BenchmarkResult {
	factory := NewEncoderFactory()
	results := make(map[EncodingType]*BenchmarkResult)

	for _, encodingType := range factory.GetSupportedTypes() {
		encoder, _ := factory.GetEncoder(encodingType)
		result := &BenchmarkResult{
			EncodingType: encodingType,
		}

		// 测试编码
		start := time.Now()
		var encoded []byte
		var err error
		for i := 0; i < iterations; i++ {
			encoded, err = encoder.Encode(msg)
			if err != nil {
				continue
			}
		}
		result.EncodeTime = time.Since(start) / time.Duration(iterations)
		result.EncodedSize = len(encoded)

		// 测试解码
		start = time.Now()
		for i := 0; i < iterations; i++ {
			_, err = encoder.Decode(encoded)
			if err != nil {
				continue
			}
		}
		result.DecodeTime = time.Since(start) / time.Duration(iterations)

		// 计算压缩比（相对于JSON）
		if encodingType != EncodingJSON {
			jsonEncoder, _ := factory.GetEncoder(EncodingJSON)
			jsonData, _ := jsonEncoder.Encode(msg)
			result.CompressionRatio = float64(len(jsonData)) / float64(len(encoded))
		} else {
			result.CompressionRatio = 1.0
		}

		results[encodingType] = result
	}

	return results
}

// CreateTestMessage 创建测试消息
func CreateTestMessage() *Message {
	return &Message{
		Version:        "1.0",
		Type:           "message",
		ID:             "test-message-id-12345",
		SenderID:       "user-abc-123",
		RecipientID:    "user-def-456",
		Content:        "这是一条测试消息，包含中文字符和emoji 😀🎉",
		Timestamp:      time.Now().Unix(),
		ConversationID: "conv-xyz-789",
		IsGroup:        false,
		Status:         "sent",
		Metadata: map[string]string{
			"client_version": "1.2.3",
			"platform":       "web",
			"user_agent":     "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)",
		},
	}
}
