package connection

import (
	"cursorIM/internal/protocol"
)

// EnhancedConnection 增强的连接接口，支持协议适配
type EnhancedConnection interface {
	Connection

	// GetProtocolType 获取连接使用的协议类型
	GetProtocolType() protocol.ProtocolType

	// SendMessageWithProtocol 使用指定协议发送消息
	SendMessageWithProtocol(message *protocol.Message, protocolType protocol.ProtocolType) error

	// SetMessageAdapter 设置消息适配器
	SetMessageAdapter(adapter *protocol.MessageAdapter)
}

// ProtocolAwareConnection 协议感知的连接基础结构
type ProtocolAwareConnection struct {
	adapter      *protocol.MessageAdapter
	protocolType protocol.ProtocolType
}

// NewProtocolAwareConnection 创建协议感知连接
func NewProtocolAwareConnection(connectionType string) *ProtocolAwareConnection {
	adapter := protocol.NewMessageAdapter()
	protocolType := adapter.GetProtocolTypeFromConnection(connectionType)

	return &ProtocolAwareConnection{
		adapter:      adapter,
		protocolType: protocolType,
	}
}

// GetProtocolType 获取协议类型
func (p *ProtocolAwareConnection) GetProtocolType() protocol.ProtocolType {
	return p.protocolType
}

// SetMessageAdapter 设置消息适配器
func (p *ProtocolAwareConnection) SetMessageAdapter(adapter *protocol.MessageAdapter) {
	p.adapter = adapter
}

// SerializeMessage 序列化消息
func (p *ProtocolAwareConnection) SerializeMessage(msg *protocol.Message) ([]byte, error) {
	return p.adapter.SerializeMessage(msg, p.protocolType)
}

// DeserializeMessage 反序列化消息
func (p *ProtocolAwareConnection) DeserializeMessage(data []byte) (*protocol.Message, error) {
	return p.adapter.DeserializeMessage(data, p.protocolType)
}
