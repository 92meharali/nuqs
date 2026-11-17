// Package shapes draws simple shapes. It's only used by the apidocs tests.
//
// It supports:
//   - squares
//   - circles
//
// Example:
//
//	s := shapes.NewSquare(2)
package shapes

// Max is the biggest size allowed.
const Max = 100

// Default sizes.
const (
	Small = 1
	Large = 10
)

// Count says how many shapes were drawn.
var Count int

// Area returns the area of any shape.
func Area(s Shape) float64 { return s.Area() }

// Shape is anything with an area.
type Shape interface {
	Area() float64
}

// Square is a square.
type Square struct {
	Side   float64 // length of one side
	secret int
}

// NewSquare makes a square.
func NewSquare(side float64) *Square { return &Square{Side: side} }

// Area returns side times side.
func (s *Square) Area() float64 { return s.Side * s.Side }

func (s *Square) hidden() {}

func helper() {}
