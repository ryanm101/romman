package dat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidDAT(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<header>
		<name>Test System</name>
		<description>Test System Description</description>
		<version>20240101</version>
		<date>2024-01-01</date>
		<author>Test Author</author>
	</header>
	<game name="Test Game (USA)">
		<description>Test Game (USA)</description>
		<rom name="test.rom" size="1024" crc="12345678" sha1="abcdef"/>
	</game>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	assert.Equal(t, "Test System", dat.Header.Name)
	assert.Equal(t, "Test System Description", dat.Header.Description)
	assert.Equal(t, "20240101", dat.Header.Version)
	assert.Equal(t, "2024-01-01", dat.Header.Date)
	assert.Equal(t, "Test Author", dat.Header.Author)
	assert.Len(t, dat.Games, 1)
	assert.Equal(t, "Test Game (USA)", dat.Games[0].Name)
	assert.Len(t, dat.Games[0].Roms, 1)
	assert.Equal(t, "test.rom", dat.Games[0].Roms[0].Name)
	assert.Equal(t, int64(1024), dat.Games[0].Roms[0].Size)
	assert.Equal(t, "12345678", dat.Games[0].Roms[0].CRC32)
	assert.Equal(t, "abcdef", dat.Games[0].Roms[0].SHA1)
}

func TestParse_MachineElement(t *testing.T) {
	// MAME uses <machine> instead of <game>
	datXML := `<?xml version="1.0"?>
<datafile>
	<header><name>MAME</name></header>
	<machine name="pacman">
		<description>Pac-Man</description>
		<rom name="pacman.zip" size="2048"/>
	</machine>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	assert.Len(t, dat.Games, 1)
	assert.Equal(t, "pacman", dat.Games[0].Name)
}

func TestParse_MultipleGames(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<header><name>Multi</name></header>
	<game name="Game 1"><rom name="a.rom" size="100"/></game>
	<game name="Game 2"><rom name="b.rom" size="200"/></game>
	<game name="Game 3"><rom name="c.rom" size="300"/></game>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	assert.Len(t, dat.Games, 3)
	assert.Equal(t, "Game 1", dat.Games[0].Name)
	assert.Equal(t, "Game 2", dat.Games[1].Name)
	assert.Equal(t, "Game 3", dat.Games[2].Name)
}

func TestParse_MultipleRoms(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<header><name>Multi ROM</name></header>
	<game name="Multi ROM Game">
		<rom name="rom1.bin" size="100" crc="11111111"/>
		<rom name="rom2.bin" size="200" crc="22222222"/>
		<rom name="rom3.bin" size="300" crc="33333333"/>
	</game>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	require.Len(t, dat.Games, 1)
	assert.Len(t, dat.Games[0].Roms, 3)
}

func TestParse_EmptyDAT(t *testing.T) {
	datXML := `<?xml version="1.0"?><datafile></datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	assert.Empty(t, dat.Header.Name)
	assert.Empty(t, dat.Games)
}

func TestParse_NoHeader(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<game name="No Header Game"><rom name="test.rom" size="100"/></game>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	assert.Empty(t, dat.Header.Name)
	assert.Len(t, dat.Games, 1)
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse(strings.NewReader("<invalid>xml<>"))
	assert.Error(t, err)
}

func TestParse_MalformedGame(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<game>
		<rom size="not-a-number"/>
	</game>
</datafile>`

	// Size is int64, "not-a-number" should cause an error
	_, err := Parse(strings.NewReader(datXML))
	assert.Error(t, err)
}

func TestParse_AllHashTypes(t *testing.T) {
	datXML := `<?xml version="1.0"?>
<datafile>
	<header><name>Hashes</name></header>
	<game name="Hash Test">
		<rom name="test.rom" size="1024" crc="AABBCCDD" md5="0123456789abcdef0123456789abcdef" sha1="0123456789abcdef0123456789abcdef01234567"/>
	</game>
</datafile>`

	dat, err := Parse(strings.NewReader(datXML))
	require.NoError(t, err)

	rom := dat.Games[0].Roms[0]
	assert.Equal(t, "AABBCCDD", rom.CRC32)
	assert.Equal(t, "0123456789abcdef0123456789abcdef", rom.MD5)
	assert.Equal(t, "0123456789abcdef0123456789abcdef01234567", rom.SHA1)
}

func TestParseFile_NotFound(t *testing.T) {
	_, err := ParseFile("/nonexistent/path.dat")
	assert.Error(t, err)
}
