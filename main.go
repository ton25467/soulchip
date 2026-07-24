package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/soulchip/game/internal/game"
)

func main() {
	// ปรับขนาดหน้าต่างเกมภายนอกเพื่อรองรับแผนที่และแผง HUD ด้านล่าง (640x736 พิกเซล)
	ebiten.SetWindowSize(640, 736)
	ebiten.SetWindowTitle("SoulChip - Core Process (Go 1.26 + Ebitengine)")

	g := game.NewGame()

	// รันลูป Ebitengine เพื่อเริ่มประมวลผลและแสดงผลตัวเกม
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
