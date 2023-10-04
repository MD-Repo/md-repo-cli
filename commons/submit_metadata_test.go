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
	description = """
	The system is a viral human rhinovirus (HRV) capsid with a mutation N219A In VP1 (PDB:1RUF). 
	The system consists of VP1, VP2, VP3, and VP4 proteins of HRV. 
	The capsid is solvated in water (TIP3) molecules. 
	The simulations used 14 CPUs and 2 Tesla K80 GPUs on a single node. 
	Five independent replicas of the trajectories were calculated. 
	This trajectory is one of the five replicas.
	"""
	
	# List of commands run to produce simulation results.
	commands = "charmrun ++local ++p $SLURM_NTASKS ++ppn $SLURM_NTASKS 'which namd2' +setcpuaffinity +idlepoll 1rufa.inp > 1rufa.out"
	
	
	date = 2017-07-16
	lead_contributor_orcid="0000-0002-9100-4108"
	
	# A list of uniprot id's for the protiens in the simulation.
	proteins = ["P03303"]
	#pdb_id = "1RUF"
	
	[software]
	name = "NAMD"
	version = "2.12 for Linux-x86_64-ibverbs-smp-CUDA"
	
	# A list of contributors, we're using TOML's array of tables syntax...
	[[contributors]]
	name = "Amitava Roy"  # Commented fields below are optional...
	# orcid = "0000-0002-9100-4108"
	# email = "amitava.roy@umontana.edu"
	# institution = "University of Montana"
	
	[water]  
	is_present = true
	model = "TIP3" 
	density = 1.0 # density of water in kg/m^3
	
	# A list of solvents used in the simulation.
	[[solvents]]
	name = "Sodium Cloride"
	ion_concentration = 0.15  # Molarity of the solvent...
	
	# A list of ligands involved in the simulation.
	[[ligands]]
	
	
	# A list of papers associated with a simulation. Add another [[papers]] block to add another paper.
	[[papers]]
	title = "Long-distance correlations of rhinovirus capsid dynamics contribute to uncoating and antiviral activity"
	authors = "A. Roy, C. B. Post"
	journal = "Proceedings of the National Academy of Sciences"
	year = 2012
	volume = "109"
	number = "14"
	pages = "5271-5276"
	doi = "10.1073/pnas.1119174109"
`

	orcid, err := ReadOrcIDFromSubmitMetadataString(metadata)
	assert.NoError(t, err)

	assert.Equal(t, "0000-0002-9100-4108", orcid)
}
