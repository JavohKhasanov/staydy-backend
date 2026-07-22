package entity

import (
	"time"

	"github.com/google/uuid"
)

// ShopItem is a reward a center offers for coins (admin-managed).
type ShopItem struct {
	ID        uuid.UUID
	OrgID     uuid.UUID
	Name      string
	Icon      string
	Price     int
	IsActive  bool
	CreatedAt time.Time
}

// StudentShopItem is an active item as the student sees it (with whether they already own it).
type StudentShopItem struct {
	ID    uuid.UUID
	Name  string
	Icon  string
	Price int
	Owned bool
}

// LeaderRow is one ranked entry in a leaderboard.
type LeaderRow struct {
	StudentID uuid.UUID
	Name      string
	XP        int
	Coins     int
	Rank      int
	IsMe      bool
}
