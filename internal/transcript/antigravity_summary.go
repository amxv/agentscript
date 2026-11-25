package transcript

import "encoding/binary"

// jetbox_summaries_proto.pb is a map keyed by conversation UUID. Read just
// the title and workspace URIs. agy updates this file for headless and ACP
// conversations even when cache/conversation_metadata.json is stale.
func loadAgySummaries(data []byte, metadata map[string]agyMetadata) {
	for _, entry := range agyProtoBytes(data, 1) {
		id := string(agyProtoFirst(entry, 1))
		if !IsSessionID(id) {
			continue
		}
		summary := agyProtoFirst(entry, 2)
		meta := metadata[id]
		if title := string(agyProtoFirst(summary, 1)); title != "" {
			meta.Summary.Title = title
		}
		var uris []string
		for _, workspace := range agyProtoBytes(summary, 9) {
			if uri := agyProtoFirst(workspace, 1); len(uri) > 0 {
				uris = append(uris, string(uri))
			}
		}
		if len(uris) > 0 {
			meta.Summary.WorkspaceURIs = uris
		}
		metadata[id] = meta
	}
}

func agyProtoFirst(data []byte, field uint64) []byte {
	if values := agyProtoBytes(data, field); len(values) > 0 {
		return values[0]
	}
	return nil
}

// Select length-delimited fields, skipping the other protobuf wire types
// used in the summary file. A partially written value ends the scan.
func agyProtoBytes(data []byte, field uint64) [][]byte {
	var values [][]byte
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 || tag>>3 == 0 {
			break
		}
		data = data[n:]
		var size uint64
		switch tag & 7 {
		case 0:
			_, n = binary.Uvarint(data)
			if n <= 0 {
				return values
			}
			size = uint64(n)
		case 1:
			size = 8
		case 2:
			size, n = binary.Uvarint(data)
			if n <= 0 {
				return values
			}
			data = data[n:]
		case 5:
			size = 4
		default:
			return values
		}
		if size > uint64(len(data)) {
			break
		}
		if tag&7 == 2 && tag>>3 == field {
			values = append(values, data[:size])
		}
		data = data[size:]
	}
	return values
}
