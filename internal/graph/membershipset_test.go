package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/authzed/spicedb/internal/caveats"
	core "github.com/authzed/spicedb/pkg/proto/core/v1"
	v1 "github.com/authzed/spicedb/pkg/proto/dispatch/v1"
	"github.com/authzed/spicedb/pkg/tuple"
)

var invert = caveats.Invert

func caveat(name string, context map[string]any) *v1.CaveatExpression {
	s, _ := structpb.NewStruct(context)
	return wrapCaveat(
		&core.ContextualizedCaveat{
			CaveatName: name,
			Context:    s,
		})
}

func TestMembershipSetAddDirectMember(t *testing.T) {
	tcs := []struct {
		name                string
		existingMembers     map[string]*v1.CaveatExpression
		directMemberID      string
		directMemberCaveat  *v1.CaveatExpression
		expectedMembers     map[string]*v1.CaveatExpression
		hasDeterminedMember bool
	}{
		{
			"add determined member to empty set",
			nil,
			"somedoc",
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
		},
		{
			"add caveated member to empty set",
			nil,
			"somedoc",
			caveat("somecaveat", nil),
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("somecaveat", nil),
			},
			false,
		},
		{
			"add caveated member to set with other members",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("somecaveat", nil),
			},
			"anotherdoc",
			caveat("anothercaveat", nil),
			map[string]*v1.CaveatExpression{
				"somedoc":    caveat("somecaveat", nil),
				"anotherdoc": caveat("anothercaveat", nil),
			},
			false,
		},
		{
			"add non-caveated member to caveated member",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("somecaveat", nil),
			},
			"somedoc",
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
		},
		{
			"add caveated member to non-caveated member",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			"somedoc",
			caveat("somecaveat", nil),
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
		},
		{
			"add caveated member to caveated member",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			"somedoc",
			caveat("c2", nil),
			map[string]*v1.CaveatExpression{
				"somedoc": caveatOr(
					caveat("c1", nil),
					caveat("c2", nil),
				),
			},
			false,
		},
		{
			"add caveats with the same name, different args",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			"somedoc",
			caveat("c1", map[string]any{
				"hi": "hello",
			}),
			map[string]*v1.CaveatExpression{
				"somedoc": caveatOr(
					caveat("c1", nil),
					caveat("c1", map[string]any{
						"hi": "hello",
					}),
				),
			},
			false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ms := membershipSetFromMap(tc.existingMembers)
			ms.AddDirectMember(tc.directMemberID, unwrapCaveat(tc.directMemberCaveat))
			require.Equal(t, tc.expectedMembers, ms.membersByID)
			require.Equal(t, tc.hasDeterminedMember, ms.HasDeterminedMember())
			require.False(t, ms.IsEmpty())
		})
	}
}

func TestMembershipSetAddMemberViaRelationship(t *testing.T) {
	tcs := []struct {
		name                     string
		existingMembers          map[string]*v1.CaveatExpression
		resourceID               string
		resourceCaveatExpression *v1.CaveatExpression
		parentRelationship       *core.RelationTuple
		expectedMembers          map[string]*v1.CaveatExpression
		hasDeterminedMember      bool
	}{
		{
			"add determined member to empty set",
			nil,
			"somedoc",
			nil,
			tuple.MustParse("document:foo#viewer@user:tom"),
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
		},
		{
			"add caveated member to empty set",
			nil,
			"somedoc",
			caveat("somecaveat", nil),
			tuple.MustParse("document:foo#viewer@user:tom"),
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("somecaveat", nil),
			},
			false,
		},
		{
			"add determined member, via caveated relationship, to empty set",
			nil,
			"somedoc",
			nil,
			withCaveat(tuple.MustParse("document:foo#viewer@user:tom"), caveat("somecaveat", nil)),
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("somecaveat", nil),
			},
			false,
		},
		{
			"add caveated member, via caveated relationship, to empty set",
			nil,
			"somedoc",
			caveat("c1", nil),
			withCaveat(tuple.MustParse("document:foo#viewer@user:tom"), caveat("c2", nil)),
			map[string]*v1.CaveatExpression{
				"somedoc": caveatAnd(
					caveat("c2", nil),
					caveat("c1", nil),
				),
			},
			false,
		},
		{
			"add caveated member, via caveated relationship, to determined set",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			"somedoc",
			caveat("c1", nil),
			withCaveat(tuple.MustParse("document:foo#viewer@user:tom"), caveat("c2", nil)),
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
		},
		{
			"add caveated member, via caveated relationship, to caveated set",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c0", nil),
			},
			"somedoc",
			caveat("c1", nil),
			withCaveat(tuple.MustParse("document:foo#viewer@user:tom"), caveat("c2", nil)),
			map[string]*v1.CaveatExpression{
				"somedoc": caveatOr(
					caveat("c0", nil),
					caveatAnd(
						caveat("c2", nil),
						caveat("c1", nil),
					),
				),
			},
			false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ms := membershipSetFromMap(tc.existingMembers)
			ms.AddMemberViaRelationship(tc.resourceID, tc.resourceCaveatExpression, tc.parentRelationship)
			require.Equal(t, tc.expectedMembers, ms.membersByID)
			require.Equal(t, tc.hasDeterminedMember, ms.HasDeterminedMember())
		})
	}
}

func TestMembershipSetUnionWith(t *testing.T) {
	tcs := []struct {
		name                string
		set1                map[string]*v1.CaveatExpression
		set2                map[string]*v1.CaveatExpression
		expected            map[string]*v1.CaveatExpression
		hasDeterminedMember bool
		isEmpty             bool
	}{
		{
			"empty with empty",
			nil,
			nil,
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"set with empty",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"empty with set",
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"non-overlapping",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": nil,
			},
			true,
			false,
		},
		{
			"non-overlapping with caveats",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": caveat("c1", nil),
			},
			true,
			false,
		},
		{
			"overlapping without caveats",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"overlapping with single caveat",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"overlapping with multiple caveats",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveatOr(caveat("c1", nil), caveat("c2", nil)),
			},
			false,
			false,
		},
		{
			"overlapping with multiple caveats and a determined member",
			map[string]*v1.CaveatExpression{
				"somedoc":    caveat("c1", nil),
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
				"somedoc":    caveatOr(caveat("c1", nil), caveat("c2", nil)),
			},
			true,
			false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ms1 := membershipSetFromMap(tc.set1)
			ms2 := membershipSetFromMap(tc.set2)
			ms1.UnionWith(ms2.AsCheckResultsMap())
			require.Equal(t, tc.expected, ms1.membersByID)
			require.Equal(t, tc.hasDeterminedMember, ms1.HasDeterminedMember())
			require.Equal(t, tc.isEmpty, ms1.IsEmpty())
		})
	}
}

func TestMembershipSetIntersectWith(t *testing.T) {
	tcs := []struct {
		name                string
		set1                map[string]*v1.CaveatExpression
		set2                map[string]*v1.CaveatExpression
		expected            map[string]*v1.CaveatExpression
		hasDeterminedMember bool
		isEmpty             bool
	}{
		{
			"empty with empty",
			nil,
			nil,
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"set with empty",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			nil,
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"empty with set",
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"basic set with set",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"non-overlapping set with set",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"partially overlapping set with set",
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			true,
			false,
		},
		{
			"set with partially overlapping set",
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			true,
			false,
		},
		{
			"partially overlapping sets with one caveat",
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveat("c2", nil),
			},
			false,
			false,
		},
		{
			"partially overlapping sets with one caveat (other side)",
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveat("c1", nil),
			},
			false,
			false,
		},
		{
			"partially overlapping sets with caveats",
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": caveatAnd(
					caveat("c1", nil),
					caveat("c2", nil),
				),
			},
			false,
			false,
		},
		{
			"overlapping sets with caveats and a determined member",
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"thirddoc":   nil,
				"anotherdoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    nil,
				"anotherdoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
				"anotherdoc": caveatAnd(
					caveat("c1", nil),
					caveat("c2", nil),
				),
			},
			true,
			false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ms1 := membershipSetFromMap(tc.set1)
			ms2 := membershipSetFromMap(tc.set2)
			ms1.IntersectWith(ms2.AsCheckResultsMap())
			require.Equal(t, tc.expected, ms1.membersByID)
			require.Equal(t, tc.hasDeterminedMember, ms1.HasDeterminedMember())
			require.Equal(t, tc.isEmpty, ms1.IsEmpty())
		})
	}
}

func TestMembershipSetSubtract(t *testing.T) {
	tcs := []struct {
		name                string
		set1                map[string]*v1.CaveatExpression
		set2                map[string]*v1.CaveatExpression
		expected            map[string]*v1.CaveatExpression
		hasDeterminedMember bool
		isEmpty             bool
	}{
		{
			"empty with empty",
			nil,
			nil,
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"empty with set",
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"set with empty",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			nil,
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"non overlapping sets",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			true,
			false,
		},
		{
			"overlapping sets with no caveats",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"overlapping sets with first having a caveat",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{},
			false,
			true,
		},
		{
			"overlapping sets with second having a caveat",
			map[string]*v1.CaveatExpression{
				"somedoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": invert(caveat("c2", nil)),
			},
			false,
			false,
		},
		{
			"overlapping sets with both having caveats",
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c1", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveatAnd(
					caveat("c1", nil),
					invert(caveat("c2", nil)),
				),
			},
			false,
			false,
		},
		{
			"overlapping sets with both having caveats and determined member",
			map[string]*v1.CaveatExpression{
				"somedoc":    caveat("c1", nil),
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveat("c2", nil),
			},
			map[string]*v1.CaveatExpression{
				"anotherdoc": nil,
				"somedoc": caveatAnd(
					caveat("c1", nil),
					invert(caveat("c2", nil)),
				),
			},
			true,
			false,
		},
		{
			"overlapping sets with both having caveats and determined members",
			map[string]*v1.CaveatExpression{
				"somedoc":    caveat("c1", nil),
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc":    caveat("c2", nil),
				"anotherdoc": nil,
			},
			map[string]*v1.CaveatExpression{
				"somedoc": caveatAnd(
					caveat("c1", nil),
					invert(caveat("c2", nil)),
				),
			},
			false,
			false,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ms1 := membershipSetFromMap(tc.set1)
			ms2 := membershipSetFromMap(tc.set2)
			ms1.Subtract(ms2.AsCheckResultsMap())
			require.Equal(t, tc.expected, ms1.membersByID)
			require.Equal(t, tc.hasDeterminedMember, ms1.HasDeterminedMember())
			require.Equal(t, tc.isEmpty, ms1.IsEmpty())
		})
	}
}

func unwrapCaveat(ce *v1.CaveatExpression) *core.ContextualizedCaveat {
	if ce == nil {
		return nil
	}
	return ce.GetCaveat()
}

func withCaveat(tple *core.RelationTuple, ce *v1.CaveatExpression) *core.RelationTuple {
	tple.Caveat = unwrapCaveat(ce)
	return tple
}
