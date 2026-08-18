package gaussdbtype_test

import (
	"context"
	"fmt"
	"os"

	"github.com/HuaweiCloudDeveloper/gaussdb-go"
	"github.com/HuaweiCloudDeveloper/gaussdb-go/gaussdbtype"
)

// Point represents a point that may be null.
type Point struct {
	X, Y  float32 // Coordinates of point
	Valid bool
}

func (p *Point) ScanPoint(v gaussdbtype.Point) error {
	*p = Point{
		X:     float32(v.P.X),
		Y:     float32(v.P.Y),
		Valid: v.Valid,
	}
	return nil
}

func (p Point) PointValue() (gaussdbtype.Point, error) {
	return gaussdbtype.Point{
		P:     gaussdbtype.Vec2{X: float64(p.X), Y: float64(p.Y)},
		Valid: true,
	}, nil
}

func (src *Point) String() string {
	if !src.Valid {
		return "null point"
	}

	return fmt.Sprintf("%.1f, %.1f", src.X, src.Y)
}

func Example_customType() {
	conn, err := gaussdbgo.Connect(context.Background(), os.Getenv(gaussdbgo.EnvGaussdbTestDatabase))
	if err != nil {
		fmt.Printf("Unable to establish connection: %v", err)
		return
	}
	defer conn.Close(context.Background())

	p := &Point{}
	err = conn.QueryRow(context.Background(), "select null::point").Scan(p)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p)

	err = conn.QueryRow(context.Background(), "select point(1.5,2.5)").Scan(p)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(p)
	// Output:
	// null point
	// 1.5, 2.5
}
