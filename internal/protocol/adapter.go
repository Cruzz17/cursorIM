package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cursorIM/internal/protocol/pb"

	"google.golang.org/protobuf/proto"
)

// ProtocolType 定义协议类型
type ProtocolType string

const (
	ProtocolTypeJSON     ProtocolType = "json"
	ProtocolTypeProtobuf ProtocolType = "protobuf"
)

// MessageAdapter 消息适配器，用于在不同协议格式之间转换
type MessageAdapter struct{}

// NewMessageAdapter 创建新的消息适配器
func NewMessageAdapter() *MessageAdapter {
	return &MessageAdapter{}
}

// JSONToProtobuf 将 JSON 消息转换为 Protobuf 消息
func (a *MessageAdapter) JSONToProtobuf(jsonMsg *Message) (*pb.Message, error) {
	if jsonMsg == nil {
		return nil, fmt.Errorf("JSON 消息不能为空")
	}

	pbMsg := &pb.Message{
		Version:        jsonMsg.Version,
		Type:           a.stringToMessageType(jsonMsg.Type),
		StatusCode:     int32(jsonMsg.StatusCode),
		ErrorCode:      jsonMsg.ErrorCode,
		RequestId:      jsonMsg.RequestID,
		Id:             jsonMsg.ID,
		SenderId:       jsonMsg.SenderID,
		RecipientId:    jsonMsg.RecipientID,
		Content:        jsonMsg.Content,
		Timestamp:      jsonMsg.Timestamp,
		ConversationId: jsonMsg.ConversationID,
		IsGroup:        jsonMsg.IsGroup,
		GroupId:        jsonMsg.GroupID,
		Status:         a.stringToMessageStatus(jsonMsg.Status),
		HandledByLocal: jsonMsg.HandledByLocal,
	}

	// 转换错误信息
	if jsonMsg.Error.Message != "" || jsonMsg.Error.Details != nil {
		pbMsg.Error = &pb.ErrorInfo{
			Message: jsonMsg.Error.Message,
		}
		if jsonMsg.Error.Details != nil {
			if details, ok := jsonMsg.Error.Details.(string); ok {
				pbMsg.Error.Details = details
			} else {
				detailsBytes, _ := json.Marshal(jsonMsg.Error.Details)
				pbMsg.Error.Details = string(detailsBytes)
			}
		}
	}

	// 转换元数据
	if jsonMsg.Metadata != nil {
		pbMsg.Metadata = jsonMsg.Metadata
	}

	return pbMsg, nil
}

// ProtobufToJSON 将 Protobuf 消息转换为 JSON 消息
func (a *MessageAdapter) ProtobufToJSON(pbMsg *pb.Message) (*Message, error) {
	if pbMsg == nil {
		return nil, fmt.Errorf("Protobuf 消息不能为空")
	}

	jsonMsg := &Message{
		Version:        pbMsg.Version,
		Type:           a.messageTypeToString(pbMsg.Type),
		StatusCode:     int(pbMsg.StatusCode),
		ErrorCode:      pbMsg.ErrorCode,
		RequestID:      pbMsg.RequestId,
		ID:             pbMsg.Id,
		SenderID:       pbMsg.SenderId,
		RecipientID:    pbMsg.RecipientId,
		Content:        pbMsg.Content,
		Timestamp:      pbMsg.Timestamp,
		ConversationID: pbMsg.ConversationId,
		IsGroup:        pbMsg.IsGroup,
		GroupID:        pbMsg.GroupId,
		Status:         a.messageStatusToString(pbMsg.Status),
		HandledByLocal: pbMsg.HandledByLocal,
		CreatedAt:      time.Unix(pbMsg.Timestamp, 0),
		UpdatedAt:      time.Unix(pbMsg.Timestamp, 0),
	}

	// 转换错误信息
	if pbMsg.Error != nil {
		jsonMsg.Error.Message = pbMsg.Error.Message
		if pbMsg.Error.Details != "" {
			jsonMsg.Error.Details = pbMsg.Error.Details
		}
	}

	// 转换元数据
	if pbMsg.Metadata != nil {
		jsonMsg.Metadata = pbMsg.Metadata
	}

	return jsonMsg, nil
}

// SerializeMessage 根据协议类型序列化消息
func (a *MessageAdapter) SerializeMessage(msg *Message, protocolType ProtocolType) ([]byte, error) {
	switch protocolType {
	case ProtocolTypeJSON:
		return json.Marshal(msg)
	case ProtocolTypeProtobuf:
		pbMsg, err := a.JSONToProtobuf(msg)
		if err != nil {
			return nil, fmt.Errorf("转换为 Protobuf 失败: %w", err)
		}
		return proto.Marshal(pbMsg)
	default:
		return nil, fmt.Errorf("不支持的协议类型: %s", protocolType)
	}
}

// DeserializeMessage 根据协议类型反序列化消息
func (a *MessageAdapter) DeserializeMessage(data []byte, protocolType ProtocolType) (*Message, error) {
	switch protocolType {
	case ProtocolTypeJSON:
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("JSON 反序列化失败: %w", err)
		}
		return &msg, nil
	case ProtocolTypeProtobuf:
		var pbMsg pb.Message
		if err := proto.Unmarshal(data, &pbMsg); err != nil {
			return nil, fmt.Errorf("Protobuf 反序列化失败: %w", err)
		}
		return a.ProtobufToJSON(&pbMsg)
	default:
		return nil, fmt.Errorf("不支持的协议类型: %s", protocolType)
	}
}

// DetectProtocolType 自动检测协议类型
func (a *MessageAdapter) DetectProtocolType(data []byte) ProtocolType {
	// 尝试解析为 JSON
	var jsonTest interface{}
	if json.Unmarshal(data, &jsonTest) == nil {
		return ProtocolTypeJSON
	}

	// 尝试解析为 Protobuf
	var pbMsg pb.Message
	if proto.Unmarshal(data, &pbMsg) == nil {
		return ProtocolTypeProtobuf
	}

	// 默认返回 JSON
	return ProtocolTypeJSON
}

// GetProtocolTypeFromConnection 根据连接类型确定协议类型
func (a *MessageAdapter) GetProtocolTypeFromConnection(connectionType string) ProtocolType {
	switch connectionType {
	case "tcp", "tcp_ws":
		// App 端使用 Protobuf
		return ProtocolTypeProtobuf
	case "websocket":
		// Web 端使用 JSON
		return ProtocolTypeJSON
	default:
		// 默认使用 JSON
		return ProtocolTypeJSON
	}
}

// 辅助方法：字符串转换为 MessageType 枚举
func (a *MessageAdapter) stringToMessageType(typeStr string) pb.MessageType {
	switch strings.ToLower(typeStr) {
	case "message", "text":
		return pb.MessageType_MESSAGE_TYPE_TEXT
	case "image":
		return pb.MessageType_MESSAGE_TYPE_IMAGE
	case "file":
		return pb.MessageType_MESSAGE_TYPE_FILE
	case "audio":
		return pb.MessageType_MESSAGE_TYPE_AUDIO
	case "video":
		return pb.MessageType_MESSAGE_TYPE_VIDEO
	case "ping":
		return pb.MessageType_MESSAGE_TYPE_PING
	case "pong":
		return pb.MessageType_MESSAGE_TYPE_PONG
	case "status":
		return pb.MessageType_MESSAGE_TYPE_STATUS
	case "command":
		return pb.MessageType_MESSAGE_TYPE_COMMAND
	case "response":
		return pb.MessageType_MESSAGE_TYPE_RESPONSE
	case "error":
		return pb.MessageType_MESSAGE_TYPE_ERROR
	default:
		return pb.MessageType_MESSAGE_TYPE_UNKNOWN
	}
}

// 辅助方法：MessageType 枚举转换为字符串
func (a *MessageAdapter) messageTypeToString(msgType pb.MessageType) string {
	switch msgType {
	case pb.MessageType_MESSAGE_TYPE_TEXT:
		return "message"
	case pb.MessageType_MESSAGE_TYPE_IMAGE:
		return "image"
	case pb.MessageType_MESSAGE_TYPE_FILE:
		return "file"
	case pb.MessageType_MESSAGE_TYPE_AUDIO:
		return "audio"
	case pb.MessageType_MESSAGE_TYPE_VIDEO:
		return "video"
	case pb.MessageType_MESSAGE_TYPE_PING:
		return "ping"
	case pb.MessageType_MESSAGE_TYPE_PONG:
		return "pong"
	case pb.MessageType_MESSAGE_TYPE_STATUS:
		return "status"
	case pb.MessageType_MESSAGE_TYPE_COMMAND:
		return "command"
	case pb.MessageType_MESSAGE_TYPE_RESPONSE:
		return "response"
	case pb.MessageType_MESSAGE_TYPE_ERROR:
		return "error"
	default:
		return "unknown"
	}
}

// 辅助方法：字符串转换为 MessageStatus 枚举
func (a *MessageAdapter) stringToMessageStatus(statusStr string) pb.MessageStatus {
	switch strings.ToLower(statusStr) {
	case "sent":
		return pb.MessageStatus_MESSAGE_STATUS_SENT
	case "delivered":
		return pb.MessageStatus_MESSAGE_STATUS_DELIVERED
	case "read":
		return pb.MessageStatus_MESSAGE_STATUS_READ
	case "failed":
		return pb.MessageStatus_MESSAGE_STATUS_FAILED
	default:
		return pb.MessageStatus_MESSAGE_STATUS_UNKNOWN
	}
}

// 辅助方法：MessageStatus 枚举转换为字符串
func (a *MessageAdapter) messageStatusToString(status pb.MessageStatus) string {
	switch status {
	case pb.MessageStatus_MESSAGE_STATUS_SENT:
		return "sent"
	case pb.MessageStatus_MESSAGE_STATUS_DELIVERED:
		return "delivered"
	case pb.MessageStatus_MESSAGE_STATUS_READ:
		return "read"
	case pb.MessageStatus_MESSAGE_STATUS_FAILED:
		return "failed"
	default:
		return "unknown"
	}
}
