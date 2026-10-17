package mydb

// RoleSlot is one extra-role position on a game. UserID 0 means a free-form name.
type RoleSlot struct {
	ID, GameID, RoleID, Slot, UserID                int
	RoleName, FreeName, Status, UserName, UserEmail string
}
