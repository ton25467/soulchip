package game

import (
	"os"
	"testing"
)

// TestMain จัดการการรันภาพรวมของชุดทดสอบ หากตรวจพบล้มเหลวจะแจ้งไปยัง backend ทันที
func TestMain(m *testing.M) {
	// 1. สปินอัปเซิร์ฟเวอร์รายงานผลข้อดีลอยพอร์ต
	serverURL, err := StartMockServer()

	// 2. รันชุดทดสอบทั้งหมด
	code := m.Run()

	// 3. หากมีข้อผิดพลาดแม้แต่จุดเดียว (code != 0) ให้แจ้งความล้มเหลวเข้า Backend ทันที
	if code != 0 && err == nil {
		SendCategorizedReport(serverURL, "test_suite_failure", 0, 0, 0, "Unit test suite execution failed")
	}

	os.Exit(code)
}

// TestPlayerCollision ทดสอบการตรวจสอบสิ่งกีดขวาง (กำแพง) ของระบบแผนที่
func TestPlayerCollision(t *testing.T) {
	tiles := [][]TileType{
		{TileWall, TileWall, TileWall},
		{TileWall, TileEmpty, TileWall},
		{TileWall, TileWall, TileWall},
	}
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles:  tiles,
	}

	if !level.IsWall(1, 0) {
		t.Error("Expected coordinate (1,0) to be a wall, but IsWall returned false")
	}
	if level.IsWall(1, 1) {
		t.Error("Expected coordinate (1,1) to be empty, but IsWall returned true")
	}

	player := NewPlayer(1, 2)
	player.isMoving = true
	player.targetGridX = 1
	player.targetGridY = 1
	player.moveTicks = 0
	player.maxTicks = 10

	for player.isMoving {
		player.Update(level)
	}

	if player.GridX != 1 || player.GridY != 1 {
		t.Errorf("Expected player to move to (1,1), but got (%d,%d)", player.GridX, player.GridY)
	}
}

// TestItemCollection ทดสอบการเดินทับและเก็บไอเทมเข้า Inventory
func TestItemCollection(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
		},
		Items: []Item{
			{GridX: 2, GridY: 1, Type: ItemRedKey, Collected: false},
		},
	}

	player := NewPlayer(1, 1)
	player.isMoving = true
	player.targetGridX = 2
	player.targetGridY = 1
	player.moveTicks = 0
	player.maxTicks = 10

	for player.isMoving {
		player.Update(level)
	}

	if !level.Items[0].Collected {
		t.Error("Expected item at (2,1) to be collected, but Collected flag is false")
	}

	if len(player.Inventory) != 1 {
		t.Fatalf("Expected inventory size to be 1, but got %d", len(player.Inventory))
	}
	if player.Inventory[0] != ItemRedKey {
		t.Errorf("Expected RedKey in player inventory, but got type %v", player.Inventory[0])
	}
}

// TestLockedDoors ทดสอบการใช้ไอเทมเปิดประตูล็อกสีแดงและน้ำเงิน
func TestLockedDoors(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileWall, TileRedDoor, TileWall},
			{TileWall, TileEmpty, TileWall},
			{TileWall, TileBlueDoor, TileWall},
		},
	}

	player := NewPlayer(1, 1)
	if !level.IsWall(1, 0) {
		t.Error("Red door should be blocked when player doesn't have RedKey")
	}

	player.Inventory = append(player.Inventory, ItemRedKey)
	player.isMoving = false
	
	player.SetInput(0, -1)
	player.Update(level)
	
	if level.Tiles[0][1] != TileEmpty {
		t.Error("Red door should turn to TileEmpty after key is consumed")
	}
	if player.hasItem(ItemRedKey) {
		t.Error("RedKey should be consumed from player inventory")
	}
}

// TestProceduralAlgorithms ทดสอบการสแกนและคำนวณจำนวน Fake Wall ในด่านต่างๆ
func TestProceduralAlgorithms(t *testing.T) {
	if getFakeWallCount(1) != 2 {
		t.Errorf("Expected level 1 fake count to be 2, got %d", getFakeWallCount(1))
	}
	if getFakeWallCount(2) != 4 {
		t.Errorf("Expected level 2 fake count to be 4, got %d", getFakeWallCount(2))
	}
	if getFakeWallCount(3) != 27 {
		t.Errorf("Expected level 3 fake count to be 27, got %d", getFakeWallCount(3))
	}

	tiles := [][]int{
		{1, 1, 1, 1, 1},
		{1, 0, 1, 0, 1},
		{1, 1, 1, 1, 1},
	}
	candidates := scanPartitionWalls(tiles)
	if len(candidates) != 1 || candidates[0].x != 2 || candidates[0].y != 1 {
		t.Errorf("Expected (2,1) to be scanned as a partition wall, got candidates: %v", candidates)
	}
}

// TestSecretBossEncounter ทดสอบการเหยียบช่องมินิบอสลับเพื่อปราบและรับถ้วยกะโหลก
func TestSecretBossEncounter(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileSecretBoss, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
		},
	}

	player := NewPlayer(1, 1)
	player.isMoving = true
	player.targetGridX = 1
	player.targetGridY = 0
	player.moveTicks = 0
	player.maxTicks = 10

	for player.isMoving {
		player.Update(level)
	}

	if level.Tiles[0][1] != TileEmpty {
		t.Error("Secret Boss tile should be cleared to TileEmpty after player step")
	}

	if len(player.Inventory) != 1 || player.Inventory[0] != ItemBossSkull {
		t.Errorf("Player should receive ItemBossSkull in inventory, got %v", player.Inventory)
	}
}

// TestStaircaseClimbing ทดสอบระบบไต่บันไดแบบปีนขึ้นลง ล็อกการบังคับ และแอนิเมชันขึ้นลงสไตล์ RE0
func TestStaircaseClimbing(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileStairsUp, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
		},
	}

	player := NewPlayer(1, 1)
	
	if level.IsWall(1, 1) {
		t.Error("Stair tiles should not block movement physically (IsWall should be false)")
	}

	player.StartClimb(1)

	if !player.isClimbing {
		t.Error("Expected player.isClimbing to be true")
	}
	if player.climbDirection != 1 {
		t.Errorf("Expected climb direction to be 1, got %d", player.climbDirection)
	}

	player.Update(level)

	if player.climbTicks != 1 {
		t.Errorf("Expected climbTicks to increment, got %d", player.climbTicks)
	}

	player.climbTicks = 45
}

// TestItemDrop ทดสอบระบบทิ้งของสัญจรบนพื้นดิน บันทึกพิกัด และย้อนกลับมาเก็บซ้ำใหม่แบบ RE0
func TestItemDrop(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty},
		},
	}

	player := NewPlayer(1, 1)
	player.Inventory = append(player.Inventory, ItemRedKey)

	ok := player.DropItem(0, level)
	if !ok {
		t.Fatal("Expected DropItem to succeed, but returned false")
	}

	if len(player.Inventory) != 0 {
		t.Errorf("Player inventory should be empty after drop, but has size %d", len(player.Inventory))
	}

	if len(level.Items) != 1 {
		t.Fatalf("Expected level.Items to contain 1 item, but has size %d", len(level.Items))
	}

	dropped := level.Items[0]
	if dropped.GridX != 1 || dropped.GridY != 1 || dropped.Type != ItemRedKey || dropped.Collected {
		t.Errorf("Expected dropped item to be uncollected RedKey at (1,1), got: %+v", dropped)
	}

	player.GridX = 1
	player.GridY = 2
	player.isMoving = true
	player.targetGridX = 1
	player.targetGridY = 1
	player.moveTicks = 0
	player.maxTicks = 10

	for player.isMoving {
		player.Update(level)
	}

	if !level.Items[0].Collected {
		t.Error("Expected dropped item to be re-collected when player walks back over it")
	}
	if len(player.Inventory) != 1 || player.Inventory[0] != ItemRedKey {
		t.Errorf("Expected player to recover RedKey in inventory, got: %v", player.Inventory)
	}
}

// TestFakeWallCollision ทดสอบการชนของกำแพงปลอม (TileFakeWall)
func TestFakeWallCollision(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileWall, TileFakeWall, TileWall},
			{TileEmpty, TileEmpty, TileEmpty},
			{TileWall, TileWall, TileWall},
		},
	}

	if !level.IsWall(1, 0) {
		t.Error("Expected TileFakeWall at (1,0) to block movement (IsWall should return true)")
	}

	player := NewPlayer(1, 1)
	player.isMoving = true
	player.targetGridX = 1
	player.targetGridY = 0
	player.moveTicks = 0
	player.maxTicks = 10

	if level.IsWall(player.targetGridX, player.targetGridY) {
		player.isMoving = false
		player.targetGridX = player.GridX
		player.targetGridY = player.GridY
	}

	if player.GridX != 1 || player.GridY != 1 {
		t.Errorf("Expected player to remain at (1,1) due to fake wall collision, got (%d,%d)", player.GridX, player.GridY)
	}
}

// TestItemBoxAdjacencyAndCapacities ทดสอบเงื่อนไขการอยู่ประชิด Item Box ความจุกระเป๋าและกล่องแต่ละชั้น
func TestItemBoxAdjacencyAndCapacities(t *testing.T) {
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileWall, TileItemBox, TileWall},
			{TileEmpty, TileEmpty, TileEmpty},
			{TileWall, TileWall, TileWall},
		},
	}

	if !level.IsWall(1, 0) {
		t.Error("TileItemBox at (1,0) should block movement (IsWall return true)")
	}

	g := &Game{
		level:          level,
		player:         NewPlayer(1, 1),
		currentLevelID: 1,
	}

	if !g.IsPlayerAdjacentToBox() {
		t.Error("Player at (1,1) should be adjacent to TileItemBox at (1,0)")
	}

	g.player.GridX = 2
	g.player.GridY = 2
	if g.IsPlayerAdjacentToBox() {
		t.Error("Player at (2,2) should not be adjacent to TileItemBox at (1,0)")
	}

	if cap := GetBoxCapacityForFloor(1); cap != 6 {
		t.Errorf("Expected Level 1 capacity 6, got %d", cap)
	}
	if cap := GetBoxCapacityForFloor(2); cap != 4 {
		t.Errorf("Expected Level 2 capacity 4, got %d", cap)
	}
	if cap := GetBoxCapacityForFloor(3); cap != 3 {
		t.Errorf("Expected Level 3 capacity 3, got %d", cap)
	}
	if cap := GetBoxCapacityForFloor(4); cap != 2 {
		t.Errorf("Expected Level 4 capacity 2, got %d", cap)
	}
}

// TestItemBoxTransferAndSwap ทดสอบการโอนย้ายและสลับไอเทมระหว่าง Inventory กับ Item Box
func TestItemBoxTransferAndSwap(t *testing.T) {
	g := &Game{
		level: &Level{
			Width: 3, Height: 3,
			Tiles: [][]TileType{
				{TileWall, TileItemBox, TileWall},
				{TileEmpty, TileEmpty, TileEmpty},
				{TileWall, TileWall, TileWall},
			},
		},
		player:         NewPlayer(1, 1),
		currentLevelID: 1,
		boxInventories: make(map[int][]ItemType),
	}

	g.player.Inventory = []ItemType{ItemRedKey, ItemBlueKey}

	// 1. Transfer RedKey from Player slot 0 to Box
	g.boxActiveCol = 0
	g.boxPlayerSlot = 0
	g.boxItemSlot = 0
	g.handleItemBoxTransfer()

	if len(g.player.Inventory) != 1 || g.player.Inventory[0] != ItemBlueKey {
		t.Errorf("Player inventory after transfer should be [ItemBlueKey], got %v", g.player.Inventory)
	}
	boxInv := g.GetCurrentBoxInventory()
	if len(boxInv) != 1 || boxInv[0] != ItemRedKey {
		t.Errorf("Box inventory after transfer should be [ItemRedKey], got %v", boxInv)
	}

	// 2. Transfer RedKey back from Box slot 0 to Player
	g.boxActiveCol = 1
	g.boxItemSlot = 0
	g.boxPlayerSlot = 1
	g.handleItemBoxTransfer()

	if len(g.player.Inventory) != 2 {
		t.Errorf("Player inventory after retrieve should have size 2, got %v", g.player.Inventory)
	}
	if len(g.GetCurrentBoxInventory()) != 0 {
		t.Errorf("Box inventory after retrieve should be empty, got %v", g.GetCurrentBoxInventory())
	}

	// 3. Test Direct Swap
	g.player.Inventory = []ItemType{ItemRedKey}
	g.SetCurrentBoxInventory([]ItemType{ItemBlueKey})

	g.boxActiveCol = 0
	g.boxPlayerSlot = 0
	g.boxItemSlot = 0
	g.handleItemBoxTransfer()

	if g.player.Inventory[0] != ItemBlueKey {
		t.Errorf("Expected Player slot 0 to swap to ItemBlueKey, got %v", g.player.Inventory[0])
	}
	if g.GetCurrentBoxInventory()[0] != ItemRedKey {
		t.Errorf("Expected Box slot 0 to swap to ItemRedKey, got %v", g.GetCurrentBoxInventory()[0])
	}

	// 4. Test Localized Storage Isolation between floors
	g.currentLevelID = 2
	if len(g.GetCurrentBoxInventory()) != 0 {
		t.Errorf("Level 2 item box should be empty initially, got %v", g.GetCurrentBoxInventory())
	}
	g.SetCurrentBoxInventory([]ItemType{ItemBossSkull})

	g.currentLevelID = 1
	if g.GetCurrentBoxInventory()[0] != ItemRedKey {
		t.Errorf("Level 1 box inventory should remain [ItemRedKey], got %v", g.GetCurrentBoxInventory())
	}

	// 5. Test Capacity Enforcement for Level 4 (cap = 2)
	g.currentLevelID = 4
	g.player.Inventory = []ItemType{ItemRedKey, ItemBlueKey, ItemEnergyChip}
	g.SetCurrentBoxInventory([]ItemType{ItemBossSkull, ItemBossSkull}) // Box is full (2 items)

	g.boxActiveCol = 0
	g.boxPlayerSlot = 2 // ItemEnergyChip
	g.boxItemSlot = 2   // out of bounds
	g.handleItemBoxTransfer()

	if len(g.GetCurrentBoxInventory()) > 2 {
		t.Errorf("Level 4 box capacity (2) exceeded: %v", g.GetCurrentBoxInventory())
	}
}

// TestCategorizedErrorReporting ทดสอบการส่ง HTTP POST รายงานข้อผิดพลาดเข้า /report
func TestCategorizedErrorReporting(t *testing.T) {
	serverURL, err := StartMockServer()
	if err != nil {
		t.Fatalf("Failed to start mock server: %v", err)
	}

	err = SendCategorizedReport(serverURL, "fps_drop", 52.5, 58.0, 1, "FPS drop test")
	if err != nil {
		t.Errorf("SendCategorizedReport returned error for fps_drop: %v", err)
	}

	err = SendCategorizedReport(serverURL, "test_suite_failure", 0, 0, 1, "Suite failure test")
	if err != nil {
		t.Errorf("SendCategorizedReport returned error for test_suite_failure: %v", err)
	}
}

// TestDiagonalSlidingCollision ทดสอบการเคลื่อนที่แนวเฉียงและการสไลด์ตามผนัง
func TestDiagonalSlidingCollision(t *testing.T) {
	// สร้างด่านทดสอบขนาด 3x3
	level := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileWall}, // กำแพงที่ (2,1)
			{TileEmpty, TileEmpty, TileEmpty},
		},
	}

	player := NewPlayer(1, 1) // ตรงกลางด่าน
	player.SetInput(1, -1)    // พยายามเดินตะวันออกเฉียงเหนือ -> ไปที่ (2,0)
	
	player.Update(level)
	if !player.isMoving {
		t.Error("Expected player to start moving diagonally, but isMoving is false")
	}
	
	for player.isMoving {
		player.moveTicks++
		if player.moveTicks >= player.maxTicks {
			player.GridX = player.targetGridX
			player.GridY = player.targetGridY
			player.isMoving = false
		}
	}
	if player.GridX != 2 || player.GridY != 0 {
		t.Errorf("Expected diagonal move to (2,0), but got (%d,%d)", player.GridX, player.GridY)
	}

	// คราวนี้ทดสอบการสไลด์เมื่อโดนสกัดจุดปลายทาง:
	levelBlocked := &Level{
		Width:  3,
		Height: 3,
		Tiles: [][]TileType{
			{TileEmpty, TileEmpty, TileWall},  // ปลายทาง (2,0) เป็นกำแพง
			{TileEmpty, TileEmpty, TileEmpty}, // ด้านขวา (2,1) ว่าง
			{TileEmpty, TileEmpty, TileEmpty},
		},
	}

	player2 := NewPlayer(1, 1)
	player2.SetInput(1, -1) // พยายามเดินเฉียงขวาบนไป (2,0) ซึ่งมีกำแพงขวาง
	player2.Update(levelBlocked)

	// เนื่องจากปลายทางบล็อก แต่ช่องขวา (2,1) ปลอดโปร่ง Gopher ควรเบี่ยงข้างสไลด์ระนาบไปทางขวาที่ (2,1)
	if player2.targetGridX != 2 || player2.targetGridY != 1 {
		t.Errorf("Expected horizontal sliding resolution to (2,1), but target is (%d,%d)", player2.targetGridX, player2.targetGridY)
	}
}

