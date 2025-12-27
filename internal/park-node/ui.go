package parknode

import (
	"p2p-park/internal/p2p"
	"p2p-park/internal/uiutil"
)

func shortID(id string) string                { return uiutil.ShortID(id) }
func formatName(name, fallback string) string { return uiutil.FormatName(name, fallback) }

const (
	ansiDim   = uiutil.AnsiDim
	ansiReset = uiutil.AnsiReset
)

func PrintBanner(p Printer, n *p2p.Node) {
	p.Println()
	p.Println("Node started.")
	p.Printf("Name:           %s\n", n.Name())
	p.Printf("ID:             %s\n", shortID(n.ID()))
	p.Printf("Addr:           %s\n", n.ListenAddr())
	p.Println()
	PrintCommands(p)
	p.Println()
}

func PrintCommands(p Printer) {
	p.Println("Commands:")
	p.Println("    /say <message>               - broadcast a chat-like message")
	p.Println("    /add <delta>                 - (dev) add points to yourself")
	p.Println("    /me                          - prints your info")
	p.Println("    /points                      - show current scores")
	p.Println("    /lb                          - show grant-based leaderboard")
	p.Println("    /quizask <pts> <ttl_s> <question> | <answer>  - create a quiz (answer not broadcast)")
	p.Println("    /quizzes                     - list open quizzes")
	p.Println("    /quizanswer <quiz_id> <answer>              - answer a quiz")
	p.Println("    /peers                       - show connected peers")
	p.Println("    /mkchan <name>               - make an encrypted channel")
	p.Println("    /joinchan <name> <hexkey>    - join an encrypted channel")
	p.Println("    /encsay <chan> <message>     - encrypted broadcast to channel")
	p.Println("    /quit                        - exit")
}
