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
	ligands = [ ]
	solvents = [ ]
	papers = [ ]
	contributors = [ ]
	simulation_permissions = [ ]

	[initial]
	short_description = "test"
	date = "2025-08-26"
	lead_contributor_orcid = "0000-0001-7374-1561"
	simulation_is_restricted = false

	[software]
	name = "test1"

	[replicates]
	replicate = nan
	total_replicates = nan

	[water]
	is_present = false
	density = nan
	water_density_units = "g/m^3"

	[[proteins]]
	molecule_id_type = "Uniprot"
	molecule_id = "x33433"

	[forcefield]

	[temperature]
	temperature = 1_100

	[protonation_method]
	protonation_method = ""

	[timestep_information]
	integration_time_step = 100

	[required_files]
	trajectory_file_name = "output.filtered.xtc"
	structure_file_name = "filtered.pdb"
	topology_file_name = "structure.prmtop"

	[[additional_files]]
	additional_file_type = "Input"
	additional_file_name = "i1"
	additional_file_description = "i1_desc"

	[[additional_files]]
	additional_file_type = "Input"
	additional_file_name = "i2"
	additional_file_description = "i2_desc"

	[[additional_files]]
	additional_file_type = "Trajectory"
	additional_file_name = "t1"
	additional_file_description = "t1_desc"
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
