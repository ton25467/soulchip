# Original User Request

## Initial Request — 2026-07-23T18:10:55Z

A Go and Ebitengine-based 3D survival horror puzzle game featuring software-based 3D perspective projection, procedural levels, and automated performance and logic testing.

Working directory: f:/soulchip
Integrity mode: benchmark

## Requirements

### R1. Localized Capacity Item Box System
Implement 3D-positioned wood/iron item boxes (TileItemBox, value 10 in map tiles) that the player can interact with.
- The player opens the box by standing adjacent to it and pressing Space or Enter.
- Item boxes must have floor-specific capacities: Level 1 (6 slots), Level 2 (4 slots), Level 3 (3 slots), Level 4 (2 slots).
- Storage contents must be localized (each box maintains separate inventory, not shared across floors).
- Create a Split HUD interface: Player inventory on the left (5 slots), Item Box inventory on the right.
- Control interface: A/D key to switch columns, W/S to select slot, Enter/Space to transfer/swap items, and Q to close.

### R2. Mansion Levels and Smooth Camera/Character Flow
Ensure all 4 mansion floors are integrated with the Staircase system (TileStairsUp and TileStairsDown) for floor transitions.
- Player character (Gopher) must not walk through walls, fake walls, or out of map bounds.
- Player movement and 3D camera follow (Lerp) must be smooth and lag-free.

### R3. Performance Diagnostics and Categorized Error Reporting
- Display a performance diagnostic overlay (showing FPS and TPS) in the HUD.
- The game and the test suite must send HTTP POST categorized error reports (e.g. `test_suite_failure`, `fps_drop`) to the mock backend server's `/report` endpoint upon detecting failures or performance drops below 60 FPS/TPS.

## Acceptance Criteria

### Item Box System
- [ ] Wood/iron item boxes are present in maps and open the split HUD when accessed.
- [ ] Inventories are saved per floor and strictly limited to the floor-specific capacity (6, 4, 3, 2 slots).
- [ ] Moving items between player inventory and box inventory functions correctly without duplication.

### Performance & Safety
- [ ] HUD displays real-time FPS and TPS.
- [ ] Dropping below 60 FPS/TPS triggers an HTTP POST request containing performance category error metrics to `/report`.
- [ ] Unit tests pass successfully under `go test ./...` with no compilation or logic failures.

## Verification Resources
- Existing test suite at internal/game/game_test.go
- Automated testing guide at autotesting.md
