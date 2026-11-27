package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExternalFormRetainsInputWhenMixedPortIsInvalid(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	for _, input := range []string{"bad", "0", "65536"} {
		s, _ := (State{}).editForm("external")
		s.fields[0].SetValue("https://fixture.example")
		s.fields[1].SetValue(" fixture secret ")
		s.fields[2].SetValue(input)
		s, cmd := s.updateForm(coreKey("enter"))
		require.True(t, s.Editing)
		require.Error(t, s.formErr)
		require.Nil(t, cmd)
		require.Equal(t, " fixture secret ", s.fields[1].Value())
		require.Equal(t, input, s.fields[2].Value())
		s.fields[2].SetValue("7891")
		s, cmd = s.updateForm(coreKey("enter"))
		defer s.Close()
		require.False(t, s.Editing)
		require.True(t, s.Busy)
		require.NotNil(t, cmd)
	}
}

func TestExternalFormAllowsBlankPortWhenClearingControllerAddress(t *testing.T) {
	t.Setenv("MIHOSH_CONFIG_DIR", t.TempDir())
	s, _ := (State{}).editForm("external")
	defer s.Close()
	s.fields[0].SetValue("")
	s.fields[2].SetValue("")
	s, cmd := s.updateForm(coreKey("enter"))
	require.False(t, s.Editing)
	require.NoError(t, s.formErr)
	require.True(t, s.Busy)
	require.Equal(t, "connect", s.task)
	require.NotNil(t, cmd)
}
