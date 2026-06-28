package outclient

import "github.com/VerTox/l2go/pkg/l2pkt"

// DefaultNpcHtml is the standard L2 placeholder for NPCs without configured dialogue.
// &$1536; resolves to "I have nothing to say to you." in the client.
const DefaultNpcHtml = `<html><body>&$1536;<br></body></html>`

// BuildNpcHtmlMessage builds the NpcHtmlMessage packet (0x19).
// Sent to the client to open an NPC dialogue HTML window.
func BuildNpcHtmlMessage(npcObjID int32, html string) []byte {
	w := l2pkt.NewWriter()
	w.WriteC(0x19)
	w.WriteD(npcObjID) // NPC object ID
	w.WriteS(html)     // HTML content (UTF-16LE)
	w.WriteD(0)        // item ID (0 for normal dialogue)
	return w.Bytes()
}
