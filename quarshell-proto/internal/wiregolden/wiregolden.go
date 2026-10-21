// Package wiregolden builds a deterministic snapshot of the generated
// OpenShell protobuf wire surface: services, methods, messages, field numbers,
// oneofs, enums and reserved ranges. It deliberately excludes comments,
// options and source locations so the snapshot only tracks wire identity.
package wiregolden

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// SchemaVersion is the snapshot format version.
const SchemaVersion = 1

// Snapshot is the canonical wire surface of all registered OpenShell files.
type Snapshot struct {
	SchemaVersion int    `json:"schema_version"`
	Files         []File `json:"files"`
}

// File is one OpenShell protobuf file.
type File struct {
	Path     string    `json:"path"`
	Package  string    `json:"package"`
	Syntax   string    `json:"syntax"`
	Services []Service `json:"services,omitempty"`
	Messages []Message `json:"messages,omitempty"`
	Enums    []Enum    `json:"enums,omitempty"`
}

// Service is one protobuf service.
type Service struct {
	Name    string   `json:"name"`
	Methods []Method `json:"methods"`
}

// Method is one unary or streaming RPC.
type Method struct {
	Name            string `json:"name"`
	Input           string `json:"input"`
	Output          string `json:"output"`
	ClientStreaming bool   `json:"client_streaming"`
	ServerStreaming bool   `json:"server_streaming"`
}

// Message is one protobuf message, including nested messages flattened by full name.
type Message struct {
	Name           string          `json:"name"`
	MapEntry       bool            `json:"map_entry,omitempty"`
	Fields         []Field         `json:"fields,omitempty"`
	Oneofs         []Oneof         `json:"oneofs,omitempty"`
	ReservedRanges []ReservedRange `json:"reserved_ranges,omitempty"`
	ReservedNames  []string        `json:"reserved_names,omitempty"`
}

// Field is one protobuf field with its wire number and kind.
type Field struct {
	Number         int32  `json:"number"`
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	Type           string `json:"type"`
	JSONName       string `json:"json_name"`
	Oneof          string `json:"oneof,omitempty"`
	Proto3Optional bool   `json:"proto3_optional,omitempty"`
	Map            bool   `json:"map,omitempty"`
}

// Oneof is one protobuf oneof.
type Oneof struct {
	Name   string   `json:"name"`
	Fields []string `json:"fields"`
}

// ReservedRange is one reserved number range.
type ReservedRange struct {
	Start int32 `json:"start"`
	End   int32 `json:"end"`
}

// Enum is one protobuf enum.
type Enum struct {
	Name   string      `json:"name"`
	Values []EnumValue `json:"values"`
}

// EnumValue is one enum member.
type EnumValue struct {
	Name   string `json:"name"`
	Number int32  `json:"number"`
}

// SnapshotRegistry returns the canonical snapshot of all registered
// OpenShell protobuf files.
func SnapshotRegistry() ([]byte, error) {
	files := make([]File, 0, 16)
	var rangeErr error
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if !strings.HasPrefix(string(fd.Package()), "openshell.") {
			return true
		}
		file, err := snapshotFile(fd)
		if err != nil {
			rangeErr = err
			return false
		}
		files = append(files, file)
		return true
	})
	if rangeErr != nil {
		return nil, rangeErr
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return marshal(Snapshot{SchemaVersion: SchemaVersion, Files: files})
}

// Read returns the canonical snapshot of a golden file.
func Read(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("parse wire golden: %w", err)
	}
	return marshal(snapshot)
}

// Equal reports whether two canonical snapshots are identical.
func Equal(left, right []byte) bool {
	return bytes.Equal(left, right)
}

func snapshotFile(fd protoreflect.FileDescriptor) (File, error) {
	file := File{
		Path:    fd.Path(),
		Package: string(fd.Package()),
		Syntax:  fd.Syntax().String(),
	}
	for i := 0; i < fd.Services().Len(); i++ {
		file.Services = append(file.Services, snapshotService(fd.Services().Get(i)))
	}
	sort.Slice(file.Services, func(i, j int) bool { return file.Services[i].Name < file.Services[j].Name })
	collectMessages(fd.Messages(), &file.Messages)
	collectEnums(fd.Enums(), &file.Enums)
	sort.Slice(file.Messages, func(i, j int) bool { return file.Messages[i].Name < file.Messages[j].Name })
	sort.Slice(file.Enums, func(i, j int) bool { return file.Enums[i].Name < file.Enums[j].Name })
	return file, nil
}

func snapshotService(sd protoreflect.ServiceDescriptor) Service {
	service := Service{Name: string(sd.FullName())}
	for i := 0; i < sd.Methods().Len(); i++ {
		method := sd.Methods().Get(i)
		service.Methods = append(service.Methods, Method{
			Name:            string(method.Name()),
			Input:           string(method.Input().FullName()),
			Output:          string(method.Output().FullName()),
			ClientStreaming: method.IsStreamingClient(),
			ServerStreaming: method.IsStreamingServer(),
		})
	}
	sort.Slice(service.Methods, func(i, j int) bool { return service.Methods[i].Name < service.Methods[j].Name })
	return service
}

func collectMessages(messages protoreflect.MessageDescriptors, out *[]Message) {
	for i := 0; i < messages.Len(); i++ {
		message := messages.Get(i)
		*out = append(*out, snapshotMessage(message))
		collectMessages(message.Messages(), out)
	}
}

func snapshotMessage(md protoreflect.MessageDescriptor) Message {
	message := Message{
		Name:     string(md.FullName()),
		MapEntry: md.IsMapEntry(),
	}
	for i := 0; i < md.Fields().Len(); i++ {
		field := md.Fields().Get(i)
		message.Fields = append(message.Fields, snapshotField(field))
	}
	sort.Slice(message.Fields, func(i, j int) bool { return message.Fields[i].Number < message.Fields[j].Number })
	for i := 0; i < md.Oneofs().Len(); i++ {
		oneof := md.Oneofs().Get(i)
		entry := Oneof{Name: string(oneof.Name())}
		for j := 0; j < oneof.Fields().Len(); j++ {
			entry.Fields = append(entry.Fields, string(oneof.Fields().Get(j).Name()))
		}
		sort.Strings(entry.Fields)
		message.Oneofs = append(message.Oneofs, entry)
	}
	sort.Slice(message.Oneofs, func(i, j int) bool { return message.Oneofs[i].Name < message.Oneofs[j].Name })
	ranges := md.ReservedRanges()
	for i := 0; i < ranges.Len(); i++ {
		item := ranges.Get(i)
		message.ReservedRanges = append(message.ReservedRanges, ReservedRange{Start: int32(item[0]), End: int32(item[1])})
	}
	sort.Slice(message.ReservedRanges, func(i, j int) bool {
		if message.ReservedRanges[i].Start != message.ReservedRanges[j].Start {
			return message.ReservedRanges[i].Start < message.ReservedRanges[j].Start
		}
		return message.ReservedRanges[i].End < message.ReservedRanges[j].End
	})
	names := md.ReservedNames()
	for i := 0; i < names.Len(); i++ {
		message.ReservedNames = append(message.ReservedNames, string(names.Get(i)))
	}
	sort.Strings(message.ReservedNames)
	return message
}

func snapshotField(field protoreflect.FieldDescriptor) Field {
	result := Field{
		Number:         int32(field.Number()),
		Name:           string(field.Name()),
		Kind:           field.Cardinality().String(),
		Type:           fieldType(field),
		JSONName:       field.JSONName(),
		Proto3Optional: field.HasOptionalKeyword(),
		Map:            field.IsMap(),
	}
	if oneof := field.ContainingOneof(); oneof != nil && !oneof.IsSynthetic() {
		result.Oneof = string(oneof.Name())
	}
	return result
}

func fieldType(field protoreflect.FieldDescriptor) string {
	if field.IsMap() {
		return "map<" + fieldDescriptorType(field.MapKey()) + "," + fieldDescriptorType(field.MapValue()) + ">"
	}
	return fieldDescriptorType(field)
}

func fieldDescriptorType(field protoreflect.FieldDescriptor) string {
	switch field.Kind() {
	case protoreflect.MessageKind, protoreflect.GroupKind:
		return string(field.Message().FullName())
	case protoreflect.EnumKind:
		return string(field.Enum().FullName())
	default:
		return field.Kind().String()
	}
}

func collectEnums(enums protoreflect.EnumDescriptors, out *[]Enum) {
	for i := 0; i < enums.Len(); i++ {
		enum := enums.Get(i)
		entry := Enum{Name: string(enum.FullName())}
		for j := 0; j < enum.Values().Len(); j++ {
			value := enum.Values().Get(j)
			entry.Values = append(entry.Values, EnumValue{Name: string(value.Name()), Number: int32(value.Number())})
		}
		sort.Slice(entry.Values, func(i, j int) bool {
			if entry.Values[i].Number != entry.Values[j].Number {
				return entry.Values[i].Number < entry.Values[j].Number
			}
			return entry.Values[i].Name < entry.Values[j].Name
		})
		*out = append(*out, entry)
	}
}

func marshal(value any) ([]byte, error) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}
