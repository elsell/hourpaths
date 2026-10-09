package profilepicture

const MaxUploadBytes = 10_000_000
const PreviewEdge = 1024
const PictureEdge = 512

type Crop struct{ X, Y, Size int }

func (c Crop) Valid(width, height int) bool {
	return c.Size > 0 && c.X >= 0 && c.Y >= 0 && c.Size <= width && c.Size <= height && c.X <= width-c.Size && c.Y <= height-c.Size
}

type Prepared struct {
	JPEG          []byte
	Width, Height int
}
