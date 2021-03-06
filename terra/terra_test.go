package terra

import (
	"fmt"
	"os"
	"testing"

	remoteexec "github.com/hashicorp/terraform/builtin/provisioners/remote-exec"
	"github.com/johandry/terranova"
	"github.com/stretchr/testify/assert"
	"github.com/terraform-providers/terraform-provider-aws/aws"
	"github.com/terraform-providers/terraform-provider-random/random"
)

func TestClient_CreateDirInTempFailure(t *testing.T) {
	TempDirPathLocation = ".mjolnirTest"
	client := Client{}
	tempDirName := "dummy/invalid"
	fullTempDirPath := TempDirPathLocation + "/" + tempDirName
	dirPath, err := client.CreateDirInTemp(tempDirName)
	assert.Nil(t, err)
	assert.Equal(t, fullTempDirPath, dirPath)
	assert.DirExists(t, fullTempDirPath)
	err = os.RemoveAll(TempDirPathLocation)
	assert.Nil(t, err)
	TempDirPathLocation = TempDirPath
}

func TestClient_ApplyCombinedFailure(t *testing.T) {
	client := createTestedDefaultClient(t)
	assert.IsType(t, Client{}, client)

	variables := make(map[string]interface{})
	// Here lays type'o to stop apply on parsing tf code
	variables["key_name1"] = "dummyKey"

	combinedRecipe := CombinedRecipe{
		File: File{
			Variables: variables,
		},
	}

	err := client.ApplyCombined(combinedRecipe, false)
	assert.Error(t, err)
	assert.Equal(
		t,
		"There are no recipes within this combined recipe",
		err.Error(),
	)

	// Test that one or more of the filepaths does not exist
	filePath := "dummy.tf"
	combinedRecipe = CombinedRecipe{
		File: File{
			Variables: variables,
		},
		FilePaths: []string{
			filePath,
		},
	}
	err = client.ApplyCombined(combinedRecipe, false)
	assert.Error(t, err)
	assert.Equal(
		t,
		fmt.Sprintf("open %s: no such file or directory", filePath),
		err.Error(),
	)

	// Test that file path for default write file is invalid
	LastExecutedFileName = "/some/dummy.path/invalid"
	PrepareDummyFile(t, filePath, "")
	filePath = "dummy.tf"
	combinedRecipe = CombinedRecipe{
		File: File{
			Variables: variables,
		},
		FilePaths: []string{
			filePath,
		},
	}
	err = client.ApplyCombined(combinedRecipe, false)
	assert.Error(t, err)
	assert.Equal(
		t,
		fmt.Sprintf("open %s: no such file or directory", LastExecutedFileName),
		err.Error(),
	)
	LastExecutedFileName = LastExecutedVariablesFileName
	RemoveDummyFile(t, filePath)

	//Test that file body is not valid
	PrepareDummyFile(t, filePath, DummyRecipeBodyFail)
	err = client.ApplyCombined(combinedRecipe, false)
	assert.Error(t, err)
	assert.Equal(
		t,
		"1 error occurred:\n\t* provider.aws: 1:3: unknown variable accessed: var.region in:\n\n${var.region}\n\n",
		err.Error(),
	)

	RemoveDummyFile(t, filePath)
	RemoveDummyFile(t, LastExecutedFileName)
	removeStateFileAndRestore(t)
}

func TestClient_ApplyCombined(t *testing.T) {
	client := createTestedDefaultClient(t)
	assert.IsType(t, Client{}, client)
	filePath := "dummy.tf"
	PrepareDummyFile(t, filePath, "")
	combinedRecipe := CombinedRecipe{
		FilePaths: []string{filePath},
	}
	err := client.ApplyCombined(combinedRecipe, false)
	assert.Error(t, err)
	assert.Equal(t, "1 error occurred:\n\t* provider.aws: Not a valid region: \n\n", err.Error())

	file := File{
		Location: LastExecutedFileName,
	}

	err = file.ReadFile()
	assert.Nil(t, err)
	assert.Equal(
		t,
		fmt.Sprintf("Last executed variables in recipe: \n%s", file.Variables),
		file.Body,
	)

	RemoveDummyFile(t, filePath)
	removeStateFileAndRestore(t)
	RemoveDummyFile(t, LastExecutedFileName)
}

func TestClient_ApplyFailure(t *testing.T) {
	dummyRecipeFileName := "dummyRecipe.tf"
	file, err := os.Create(dummyRecipeFileName)
	assert.Nil(t, err)

	num, err := file.Write([]byte(DummyRecipeBodyFail))
	assert.Nil(t, err)
	assert.Equal(t, len(DummyRecipeBodyFail), num)

	client := createTestedDefaultClient(t)
	assert.IsType(t, Client{}, client)

	variables := make(map[string]interface{})
	// Here lays type'o to stop apply on parsing tf code
	variables["key_name1"] = "dummyKey"

	recipe := File{
		Location:  dummyRecipeFileName,
		Variables: variables,
	}

	err = client.Apply(recipe, false)
	assert.Error(t, err)
	assert.Equal(
		t,
		"1 error occurred:\n\t* provider.aws: 1:3: unknown variable accessed: var.region in:\n\n${var.region}\n\n",
		err.Error(),
	)

	removeStateFileAndRestore(t)
	RemoveDummyFile(t, LastExecutedFileName)
}

func TestClient_DefaultClientCreateStateFile(t *testing.T) {
	client := createTestedDefaultClient(t)
	assert.IsType(t, Client{}, client)
	assert.FileExists(t, client.state.Location)

	err := client.state.ReadFile()
	assert.Nil(t, err)

	removeStateFileAndRestore(t)
}

func TestClient_DumpVariables_NilVariables(t *testing.T) {
	client := createTestedDefaultClient(t)
	variables, err := client.DumpVariables()
	assert.Nil(t, err)
	assert.Empty(t, variables)

	removeStateFileAndRestore(t)
}

func TestClient_DumpVariables(t *testing.T) {
	StateFileName = "dummy.tfstate"
	stateFile, err := DefaultStateFile()
	assert.Nil(t, err)

	vars := make(map[string]interface{})
	vars["dummyKey"] = "dummyVar"
	vars["dummyKey1"] = "dummyVar1"

	platform := &terranova.Platform{
		Vars: vars,
	}

	client := Client{
		platform: platform,
		state:    stateFile,
	}

	variables, err := client.DumpVariables()
	assert.Empty(t, err)
	assert.Equal(t, vars, variables)

	removeStateFileAndRestore(t)
}

func TestClient_PreparePlatformFailure_RecipeDoesNotExist(t *testing.T) {
	fileName := "dummy.tf"
	client := Client{}
	file := File{
		Location: fileName,
	}
	err := client.PreparePlatform(file)
	assert.Error(t, err)
	assert.IsType(t, &os.PathError{}, err)
	assert.Equal(
		t,
		err.Error(),
		fmt.Sprintf("open %s: no such file or directory", fileName),
	)
}

func TestClient_PreparePlatformFailure_PlatformIsNotInitialized(t *testing.T) {
	fileName := "dummyRecipe.tf"
	fileBody := "dummy file body"
	PrepareDummyFile(t, fileName, fileBody)
	client := Client{}
	file := File{
		Location: fileName,
	}
	err := client.PreparePlatform(file)
	assert.Error(t, err)
	assert.IsType(t, ClientError{}, err)
	assert.Equal(t, "Platform is not initialized", err.Error())
	RemoveDummyFile(t, fileName)
}

func TestClient_PreparePlatformWithVariables(t *testing.T) {
	StateFileName = "dummy.tfstate"
	stateFile, err := DefaultStateFile()
	assert.Nil(t, err)

	fileName := "dummyRecipe.tf"
	fileBody := "dummy file body"
	PrepareDummyFile(t, fileName, fileBody)

	vars := make(map[string]interface{})
	vars["dummyKey"] = "dummyVar"
	vars["dummyKey1"] = "dummyVar1"

	newVars := make(map[string]interface{})
	vars["dummyKey"] = []string{"some", "values"}
	vars["dummyKey1"] = map[string]string{"dummySubKey1": "newValue"}

	// Join two maps
	joinedVars := newVars

	for key, value := range vars {
		joinedVars[key] = value
	}

	platform := &terranova.Platform{
		Vars: vars,
	}

	client := Client{
		platform: platform,
		state:    stateFile,
	}

	file := File{
		Location:  fileName,
		Variables: newVars,
	}

	err = client.PreparePlatform(file)
	assert.Nil(t, err)
	assert.Equal(t, client.platform.Code, fileBody)

	dumpedVariables, err := client.DumpVariables()
	assert.Nil(t, err)
	assert.Equal(t, dumpedVariables, joinedVars)

	RemoveDummyFile(t, fileName)
	removeStateFileAndRestore(t)
}

func TestClient_WriteStateToFilesFailure(t *testing.T) {
	client := Client{
		platform: &terranova.Platform{},
	}
	err := client.WriteStateToFiles(false)
	assert.Error(t, err)
	assert.IsType(t, ClientError{}, err)
	assert.Equal(t, "No state file found", err.Error())
}

func TestClient_WriteStateToFiles(t *testing.T) {
	StateFileName = "dummy.tfstate"
	StateFileBody = ProperOutputFixture
	TempDirPathLocation = ".mjolnirTest"
	stateFile, err := DefaultStateFile()
	assert.Nil(t, err)

	platform := &terranova.Platform{}
	platform, err = platform.ReadStateFromFile(StateFileName)
	assert.Nil(t, err)

	client := Client{
		platform: platform,
		state:    stateFile,
	}

	err = client.WriteStateToFiles(false)
	assert.Nil(t, err)

	outputLogFileName := TempDirPathLocation + "/quorum-bastion-cocroaches-attack/output.log"
	outputLogFile := File{
		Location: outputLogFileName,
	}
	assert.FileExists(t, TempDirPathLocation+"/quorum-bastion-cocroaches-attack/output.log")
	err = outputLogFile.ReadFile()
	assert.Nil(t, err)
	assert.Greater(t, len(outputLogFile.Body), 1)
	assert.Equal(
		t,
		fmt.Sprintf("%s%s", ColorizedOutputPrefix, OutputAsAStringWithoutHeaderFixture),
		outputLogFile.Body,
	)

	removeStateFileAndRestore(t)
}

func TestClient_DefaultClient(t *testing.T) {
	client := Client{}
	err := client.DefaultClient()
	assert.Nil(t, err)
	providers := client.platform.Providers
	provisioners := client.platform.Provisioners
	assert.NotNil(t, providers)

	provider, ok := providers["random"]
	assert.True(t, ok)
	assert.IsType(t, random.Provider(), provider)

	provider, ok = providers["aws"]
	assert.True(t, ok)
	assert.IsType(t, aws.Provider(), provider)

	provider, ok = providers["local"]
	assert.True(t, ok)
	assert.IsType(t, aws.Provider(), provider)

	provider, ok = providers["null"]
	assert.True(t, ok)
	assert.IsType(t, aws.Provider(), provider)

	provider, ok = providers["tls"]
	assert.True(t, ok)
	assert.IsType(t, aws.Provider(), provider)

	provider, ok = providers["template"]
	assert.True(t, ok)
	assert.IsType(t, aws.Provider(), provider)

	provisioner, ok := provisioners["remote-exec"]
	assert.True(t, ok)
	assert.IsType(t, remoteexec.Provisioner(), provisioner)
}

func createTestedDefaultClient(t *testing.T) Client {
	StateFileName = "dummy.tfstate"
	StateFileBody = ProperOutputFixture
	client := Client{}
	err := client.DefaultClient()
	assert.Nil(t, err)

	return client
}

func removeStateFileAndRestore(t *testing.T) {
	RemoveDummyFile(t, StateFileName)
	err := os.RemoveAll(TempDirPathLocation)
	assert.Nil(t, err)
	StateFileName = DefaulStateFileName
	StateFileBody = DefaultStateFileBody
}
