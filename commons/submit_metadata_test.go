package commons

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubmitMetadata(t *testing.T) {
	t.Run("test ReadSubmitMetadata", testReadSubmitMetadata)
}

func testReadSubmitMetadata(t *testing.T) {
	metadata := `
	short_description = "test"
	lead_contributor_orcid = "0000-0001-7374-1561"
	software_name = "test1"
	replicate_id = "XXX"
    pdb_id = "4u3n"
    uniprot_ids = ["x33433",]
	forcefield = "the force"
	protonation_method = ""
	temperature_kelvin = 1_100
	integration_time_step_fs = 100
	trajectory_file_name = "output.filtered.xtc"
	structure_file_name = "filtered.pdb"
	topology_file_name = "structure.prmtop"

	[water]
	density = nan
	water_density_units = "g/m^3"

	[[additional_files]]
	file_type = "Input"
	file_name = "i1"
	description = "i1_desc"

	[[additional_files]]
	file_type = "Input"
	file_name = "i2"
	description = "i2_desc"

	[[additional_files]]
	file_type = "Trajectory"
	file_name = "t1"
	description = "t1_desc"
`

	submitMetadata, err := ParseSubmitMetadataString(metadata)
	assert.NoError(t, err)

	orcid, err := submitMetadata.GetOrcID()
	assert.NoError(t, err)

	assert.Equal(t, "0000-0001-7374-1561", orcid)

	files := submitMetadata.GetFiles()
	assert.Equal(t, 6, len(files))

	assert.Contains(t, files, "output.filtered.xtc")
	assert.Contains(t, files, "filtered.pdb")
	assert.Contains(t, files, "structure.prmtop")
	assert.Contains(t, files, "i1")
	assert.Contains(t, files, "i2")
	assert.Contains(t, files, "t1")
}
