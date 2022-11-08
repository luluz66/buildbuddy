package parser

import (
	"testing"

	"github.com/buildbuddy-io/buildbuddy/cli/log"
	"github.com/buildbuddy-io/buildbuddy/server/testutil/testfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	log.Configure([]string{"--verbose=1"})
}

func TestParseBazelrc_Basic(t *testing.T) {
	ws := testfs.MakeTempDir(t)
	testfs.WriteAllFileContents(t, ws, map[string]string{
		"WORKSPACE":      "",
		"import.bazelrc": "",
		".bazelrc": `

# COMMENT
#ANOTHER COMMENT
#

startup --startup_flag_1
startup:config --startup_configs_are_not_supported_so_this_flag_should_be_ignored

# continuations are allowed \
--this_is_not_a_flag_since_it_is_part_of_the_previous_line

--common_global_flag_1          # trailing comments are allowed
common --common_global_flag_2
common:foo --config_foo_global_flag
common:bar --config_bar_global_flag

build --build_flag_1
build:foo --build_config_foo_flag

# Should be able to refer to the "forward_ref" config even though
# it comes later on in the file
build:foo --config=forward_ref

build:foo --build_config_foo_multi_1 --build_config_foo_multi_2

build:forward_ref --build_config_forward_ref_flag

build:bar --build_config_bar_flag

test --config=bar

import     %workspace%/import.bazelrc
try-import %workspace%/NONEXISTENT.bazelrc
`,
	})

	for _, tc := range []struct {
		args                 []string
		expectedExpandedArgs []string
	}{
		{
			[]string{"query"},
			[]string{
				"--startup_flag_1",
				"query",
				"--common_global_flag_1",
				"--common_global_flag_2",
			},
		},
		{
			[]string{"--explicit_startup_flag", "query"},
			[]string{
				"--startup_flag_1",
				"--explicit_startup_flag",
				"query",
				"--common_global_flag_1",
				"--common_global_flag_2",
			},
		},
		{
			[]string{"build"},
			[]string{
				"--startup_flag_1",
				"build",
				"--common_global_flag_1",
				"--common_global_flag_2",
				"--build_flag_1",
			},
		},
		{
			[]string{"build", "--explicit_flag"},
			[]string{
				"--startup_flag_1",
				"build",
				"--common_global_flag_1",
				"--common_global_flag_2",
				"--build_flag_1",
				"--explicit_flag",
			},
		},
		{
			[]string{"build", "--config=foo"},
			[]string{
				"--startup_flag_1",
				"build",
				"--common_global_flag_1",
				"--common_global_flag_2",
				"--build_flag_1",
				"--config_foo_global_flag",
				"--build_config_foo_flag",
				"--build_config_forward_ref_flag",
				"--build_config_foo_multi_1",
				"--build_config_foo_multi_2",
			},
		},
		{
			[]string{"build", "--config=foo", "--config", "bar"},
			[]string{
				"--startup_flag_1",
				"build",
				"--common_global_flag_1",
				"--common_global_flag_2",
				"--build_flag_1",
				"--config_foo_global_flag",
				"--build_config_foo_flag",
				"--build_config_forward_ref_flag",
				"--build_config_foo_multi_1",
				"--build_config_foo_multi_2",
				"--config_bar_global_flag",
				"--build_config_bar_flag",
			},
		},
		{
			[]string{"test"},
			[]string{
				"--startup_flag_1",
				"test",
				"--common_global_flag_1",
				"--common_global_flag_2",
				"--build_flag_1",
				"--config_bar_global_flag",
				"--build_config_bar_flag",
			},
		},
	} {
		expandedArgs, err := expandConfigs(ws, tc.args)

		require.NoError(t, err, "error expanding %s", tc.args)
		assert.Equal(t, tc.expectedExpandedArgs, expandedArgs)
	}
}

func TestParseBazelrc_CircularConfigReference(t *testing.T) {
	ws := testfs.MakeTempDir(t)
	testfs.WriteAllFileContents(t, ws, map[string]string{
		"WORKSPACE": "",
		".bazelrc": `
build:a --config=b
build:b --config=c
build:c --config=a

build:d --config=d
`,
	})

	_, err := expandConfigs(ws, []string{"build", "--config=a"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular --config reference detected: a -> b -> c -> a")

	_, err = expandConfigs(ws, []string{"build", "--config=d"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular --config reference detected: d -> d")
}

func TestParseBazelrc_CircularImport(t *testing.T) {
	ws := testfs.MakeTempDir(t)
	testfs.WriteAllFileContents(t, ws, map[string]string{
		"WORKSPACE": "",
		".bazelrc":  `import %workspace%/a.bazelrc`,
		"a.bazelrc": `import %workspace%/b.bazelrc`,
		"b.bazelrc": `import %workspace%/a.bazelrc`,
	})

	_, err := expandConfigs(ws, []string{"build"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular import detected:")

	_, err = expandConfigs(ws, []string{"build"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular import detected:")
}