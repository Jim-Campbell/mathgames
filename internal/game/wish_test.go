package game

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestService_Wish_ConflictWithoutSevenBalls(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	for i := 1; i <= 6; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockDragonBall, Ref: strconv.Itoa(i)})
	}

	_, err := svc.Wish(ctx, "krillin")
	if err == nil {
		t.Fatal("expected error with only 6 balls")
	}
	if !strings.HasPrefix(err.Error(), "conflict:") {
		t.Fatalf("expected conflict: prefix, got %q", err.Error())
	}
}

func TestService_Wish_GrantsFighterAndConsumesBalls(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	for i := 1; i <= 7; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockDragonBall, Ref: strconv.Itoa(i)})
	}

	u, err := svc.Wish(ctx, "vegeta")
	if err != nil {
		t.Fatalf("Wish: %v", err)
	}
	if u.Ref != "vegeta" || u.Kind != UnlockFighter {
		t.Fatalf("unexpected unlock: %+v", u)
	}

	balls, err := store.ListUnlocks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range balls {
		if b.Kind == UnlockDragonBall {
			t.Fatalf("dragon ball %s should have been consumed", b.Ref)
		}
	}

	bonus, err := store.GetSkillState(ctx, BonusSkillSlug)
	if err != nil {
		t.Fatal(err)
	}
	if bonus.XP != WishXP {
		t.Fatalf("bonus XP = %d, want %d", bonus.XP, WishXP)
	}
}

func TestService_Wish_UnknownFighterIsInvalid(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	for i := 1; i <= 7; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockDragonBall, Ref: strconv.Itoa(i)})
	}

	_, err := svc.Wish(ctx, "not-a-real-fighter")
	if err == nil || !strings.HasPrefix(err.Error(), "invalid:") {
		t.Fatalf("expected invalid: error, got %v", err)
	}
}
