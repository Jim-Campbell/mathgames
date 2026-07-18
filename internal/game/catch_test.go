package game

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestService_Catch_ConflictWithoutEightBadges(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	for i := 1; i <= 7; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockGymBadge, Ref: strconv.Itoa(i)})
	}

	_, err := svc.Catch(ctx, "pidgey")
	if err == nil {
		t.Fatal("expected error with only 7 badges")
	}
	if !strings.HasPrefix(err.Error(), "conflict:") {
		t.Fatalf("expected conflict: prefix, got %q", err.Error())
	}
}

func TestService_Catch_GrantsPokemonAndConsumesBadges(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()

	for i := 1; i <= 8; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockGymBadge, Ref: strconv.Itoa(i)})
	}

	u, err := svc.Catch(ctx, "gengar")
	if err != nil {
		t.Fatalf("Catch: %v", err)
	}
	if u.Ref != "gengar" || u.Kind != UnlockPokemon {
		t.Fatalf("unexpected unlock: %+v", u)
	}

	badges, err := store.ListUnlocks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range badges {
		if b.Kind == UnlockGymBadge {
			t.Fatalf("gym badge %s should have been consumed", b.Ref)
		}
	}

	bonus, err := store.GetSkillState(ctx, BonusSkillSlug)
	if err != nil {
		t.Fatal(err)
	}
	if bonus.XP != CatchXP {
		t.Fatalf("bonus XP = %d, want %d", bonus.XP, CatchXP)
	}
}

func TestService_Catch_UnknownPokemonIsInvalid(t *testing.T) {
	svc, store := testService()
	ctx := context.Background()
	for i := 1; i <= 8; i++ {
		store.InsertUnlock(ctx, &Unlock{Kind: UnlockGymBadge, Ref: strconv.Itoa(i)})
	}

	_, err := svc.Catch(ctx, "not-a-real-pokemon")
	if err == nil || !strings.HasPrefix(err.Error(), "invalid:") {
		t.Fatalf("expected invalid: error, got %v", err)
	}
}
