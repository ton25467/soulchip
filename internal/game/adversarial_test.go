package game

import (
	"testing"
)

// TestAdversarial_ItemBoxCapacityBoundaries stress-tests floor-specific item box capacity limits (R1).
func TestAdversarial_ItemBoxCapacityBoundaries(t *testing.T) {
	floors := []struct {
		levelID  int
		capacity int
	}{
		{levelID: 1, capacity: 6},
		{levelID: 2, capacity: 4},
		{levelID: 3, capacity: 3},
		{levelID: 4, capacity: 2},
	}

	for _, tc := range floors {
		t.Run("FloorCap_"+string(rune('0'+tc.levelID)), func(t *testing.T) {
			g := &Game{
				level:          NewDefaultLevel(),
				player:         NewPlayer(1, 1),
				currentLevelID: tc.levelID,
				boxInventories: make(map[int][]ItemType),
			}

			// Fill box to exact capacity
			fullBox := make([]ItemType, tc.capacity)
			for i := 0; i < tc.capacity; i++ {
				fullBox[i] = ItemRedKey
			}
			g.SetCurrentBoxInventory(fullBox)

			// Give player an extra item
			g.player.Inventory = []ItemType{ItemBlueKey}

			// Attempt 1: Player-initiated transfer to an empty/overflow slot index
			g.boxActiveCol = 0
			g.boxPlayerSlot = 0
			g.boxItemSlot = tc.capacity // points beyond current box length / capacity

			g.handleItemBoxTransfer()

			boxInv := g.GetCurrentBoxInventory()
			if len(boxInv) > tc.capacity {
				t.Fatalf("CRITICAL BUG: Level %d box capacity is %d, but inventory grew to %d after transfer attempt!",
					tc.levelID, tc.capacity, len(boxInv))
			}

			// Attempt 2: Player-initiated transfer to slot index 0 (which is occupied)
			// Should perform swap, NOT increase capacity
			g.boxItemSlot = 0
			g.handleItemBoxTransfer()

			boxInv = g.GetCurrentBoxInventory()
			if len(boxInv) > tc.capacity {
				t.Fatalf("CRITICAL BUG: Level %d box capacity is %d, but inventory grew to %d after swap attempt!",
					tc.levelID, tc.capacity, len(boxInv))
			}
		})
	}
}

// TestAdversarial_ItemSwapLogic stress-tests item swapping between full player and full box inventories (R1).
func TestAdversarial_ItemSwapLogic(t *testing.T) {
	g := &Game{
		level:          NewDefaultLevel(),
		player:         NewPlayer(1, 1),
		currentLevelID: 1,
		boxInventories: make(map[int][]ItemType),
	}

	// Full player inventory (5 items)
	playerItems := []ItemType{ItemRedKey, ItemBlueKey, ItemEnergyChip, ItemBossSkull, ItemRedKey}
	g.player.Inventory = make([]ItemType, len(playerItems))
	copy(g.player.Inventory, playerItems)

	// Full Level 1 box inventory (6 items)
	boxItems := []ItemType{ItemBlueKey, ItemEnergyChip, ItemBossSkull, ItemRedKey, ItemBlueKey, ItemEnergyChip}
	g.SetCurrentBoxInventory(boxItems)

	initialTotalCount := len(g.player.Inventory) + len(g.GetCurrentBoxInventory()) // 5 + 6 = 11

	// Perform multiple swaps between various slot combinations
	swapPairs := [][2]int{
		{0, 0}, {4, 5}, {2, 3}, {1, 4}, {3, 1},
	}

	for _, pair := range swapPairs {
		pSlot, bSlot := pair[0], pair[1]

		pItemBefore := g.player.Inventory[pSlot]
		bItemBefore := g.GetCurrentBoxInventory()[bSlot]

		g.boxActiveCol = 0
		g.boxPlayerSlot = pSlot
		g.boxItemSlot = bSlot

		g.handleItemBoxTransfer()

		pItemAfter := g.player.Inventory[pSlot]
		bItemAfter := g.GetCurrentBoxInventory()[bSlot]

		if pItemAfter != bItemBefore || bItemAfter != pItemBefore {
			t.Errorf("Swap failed for player slot %d and box slot %d: expected player got %v and box got %v, but got player %v and box %v",
				pSlot, bSlot, bItemBefore, pItemBefore, pItemAfter, bItemAfter)
		}

		currentTotalCount := len(g.player.Inventory) + len(g.GetCurrentBoxInventory())
		if currentTotalCount != initialTotalCount {
			t.Fatalf("ITEM LOSS OR DUPLICATION DETECTED: expected total items %d, got %d", initialTotalCount, currentTotalCount)
		}
	}
}

// TestAdversarial_PerFloorIsolation stress-tests isolated item box inventories across floors 1->2->3->4->3->2->1 (R1, R2).
func TestAdversarial_PerFloorIsolation(t *testing.T) {
	g := &Game{
		level:          NewDefaultLevel(),
		player:         NewPlayer(1, 1),
		currentLevelID: 1,
		boxInventories: make(map[int][]ItemType),
	}

	// Set distinct contents in boxes on floors 1..4
	g.boxInventories[1] = []ItemType{ItemRedKey}
	g.boxInventories[2] = []ItemType{ItemBlueKey, ItemBlueKey}
	g.boxInventories[3] = []ItemType{ItemEnergyChip}
	g.boxInventories[4] = []ItemType{ItemBossSkull, ItemBossSkull}

	traversal := []int{1, 2, 3, 4, 3, 2, 1}

	expectedContents := map[int][]ItemType{
		1: {ItemRedKey},
		2: {ItemBlueKey, ItemBlueKey},
		3: {ItemEnergyChip},
		4: {ItemBossSkull, ItemBossSkull},
	}

	for _, floor := range traversal {
		g.currentLevelID = floor
		currentBox := g.GetCurrentBoxInventory()
		expected := expectedContents[floor]

		if len(currentBox) != len(expected) {
			t.Fatalf("CROSS-FLOOR LEAKAGE DETECTED on Level %d: expected %d items, got %d items (%v)",
				floor, len(expected), len(currentBox), currentBox)
		}

		for i := range expected {
			if currentBox[i] != expected[i] {
				t.Fatalf("CROSS-FLOOR LEAKAGE DETECTED on Level %d slot %d: expected %v, got %v",
					floor, i, expected[i], currentBox[i])
			}
		}
	}
}

// TestAdversarial_PlayerInventoryWipeOnLoadLevel checks whether loadLevel erroneously wipes player items during floor transitions.
func TestAdversarial_PlayerInventoryWipeOnLoadLevel(t *testing.T) {
	g := &Game{
		level:          NewDefaultLevel(),
		player:         NewPlayer(1, 1),
		currentLevelID: 1,
		boxInventories: make(map[int][]ItemType),
	}

	// Give player a RedKey and BlueKey
	g.player.Inventory = []ItemType{ItemRedKey, ItemBlueKey}

	// Simulate loading Level 2 (e.g. climbing stairs)
	g.loadLevel(2, 1, 1)

	if len(g.player.Inventory) != 2 {
		t.Errorf("BUG DETECTED IN LOADLEVEL: Player inventory was wiped from 2 items down to %d items when changing floors!",
			len(g.player.Inventory))
	}
}

// TestAdversarial_CollisionAndBounds stress-tests collision for walls, fake walls, boxes, doors, and out-of-bound coords (R2).
func TestAdversarial_CollisionAndBounds(t *testing.T) {
	level := &Level{
		Width:  5,
		Height: 5,
		Tiles: [][]TileType{
			{TileWall, TileFakeWall, TileItemBox, TileRedDoor, TileBlueDoor},
			{TileEmpty, TileEmpty, TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty, TileEmpty, TileEmpty},
			{TileEmpty, TileEmpty, TileEmpty, TileEmpty, TileEmpty},
		},
	}

	// 1. Check IsWall for all solid/blocking tile types
	blockingTiles := []struct {
		x, y int
		name string
	}{
		{0, 0, "TileWall (1)"},
		{1, 0, "TileFakeWall (7)"},
		{2, 0, "TileItemBox (10)"},
		{3, 0, "TileRedDoor (4)"},
		{4, 0, "TileBlueDoor (5)"},
	}

	for _, bt := range blockingTiles {
		if !level.IsWall(bt.x, bt.y) {
			t.Errorf("COLLISION FAILURE: IsWall(%d, %d) returned false for %s, should be true!", bt.x, bt.y, bt.name)
		}
	}

	// 2. Check IsWall for Out-Of-Bounds / Negative coordinates
	oobCoords := []struct {
		x, y int
		desc string
	}{
		{-1, 0, "Negative X (-1, 0)"},
		{0, -1, "Negative Y (0, -1)"},
		{-5, -5, "Negative X and Y (-5, -5)"},
		{5, 0, "X >= Width (5, 0)"},
		{0, 5, "Y >= Height (0, 5)"},
		{100, 100, "Extreme OOB (100, 100)"},
	}

	for _, oob := range oobCoords {
		if !level.IsWall(oob.x, oob.y) {
			t.Errorf("BOUNDS FAILURE: IsWall(%d, %d) returned false for %s, should strictly return true!", oob.x, oob.y, oob.desc)
		}
	}

	// 3. Verify Player Movement into blocked tiles
	player := NewPlayer(0, 1)

	// Try moving up into TileWall (0, 0)
	player.targetGridX = 0
	player.targetGridY = 0
	if level.IsWall(player.targetGridX, player.targetGridY) {
		// Player logic correctly blocks move
	} else {
		t.Error("Player allowed to set target into TileWall!")
	}

	// Try moving into OOB negative coords (-1, 1)
	player.targetGridX = -1
	player.targetGridY = 1
	if !level.IsWall(player.targetGridX, player.targetGridY) {
		t.Error("Player allowed to set target into negative coordinate (-1, 1)!")
	}
}
