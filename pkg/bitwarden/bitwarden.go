package bitwarden

import "os"

// Field represents a field in BitWarden
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Attachment represents an attachment in BitWarden
type Attachment struct {
	ID       string `json:"id"`
	FileName string `json:"fileName"`
}

// Login represents login in BitWarden
type Login struct {
	Password string `json:"password,omitempty"`
}

// Item represents an item in BitWarden
// It has only fields we are interested in
type Item struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name"`
	Type int    `json:"type"`
	//Login does NOT exist on some BitWarden entries, e.g, secure notes.
	Login       *Login       `json:"login,omitempty"`
	Fields      []Field      `json:"fields"`
	Attachments []Attachment `json:"attachments"`
}

// Client is used to communicate with BitWarden
type Client interface {
	GetFieldOnItem(itemName, fieldName string) ([]byte, error)
	GetAttachmentOnItem(itemName, attachmentName string) ([]byte, error)
	GetPassword(itemName string) ([]byte, error)
	Logout() ([]byte, error)
	SetFieldOnItem(itemName, fieldName string, fieldValue []byte) error
	SetAttachmentOnItem(itemName, attachmentName string, fileContents []byte) error
	SetPassword(itemName string, password []byte) error
}

// NewClient generates a BitWarden client
func NewClient(username, password string, addSecret func(s string)) (Client, error) {
	return newCliClient(username, password, addSecret)
}

// NewDryRunClient generates a BitWarden client
func NewDryRunClient(outputFile *os.File) (Client, error) {
	return newDryRunClient(outputFile)
}
