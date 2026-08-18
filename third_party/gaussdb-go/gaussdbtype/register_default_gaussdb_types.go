//go:build !nogaussdbregisterdefaulttypes

package gaussdbtype

func registerDefaultGaussdbTypeVariants[T any](m *Map, name string) {
	arrayName := "_" + name

	var value T
	m.RegisterDefaultGaussdbType(value, name)  // T
	m.RegisterDefaultGaussdbType(&value, name) // *T

	var sliceT []T
	m.RegisterDefaultGaussdbType(sliceT, arrayName)  // []T
	m.RegisterDefaultGaussdbType(&sliceT, arrayName) // *[]T

	var slicePtrT []*T
	m.RegisterDefaultGaussdbType(slicePtrT, arrayName)  // []*T
	m.RegisterDefaultGaussdbType(&slicePtrT, arrayName) // *[]*T

	var arrayOfT Array[T]
	m.RegisterDefaultGaussdbType(arrayOfT, arrayName)  // Array[T]
	m.RegisterDefaultGaussdbType(&arrayOfT, arrayName) // *Array[T]

	var arrayOfPtrT Array[*T]
	m.RegisterDefaultGaussdbType(arrayOfPtrT, arrayName)  // Array[*T]
	m.RegisterDefaultGaussdbType(&arrayOfPtrT, arrayName) // *Array[*T]

	var flatArrayOfT FlatArray[T]
	m.RegisterDefaultGaussdbType(flatArrayOfT, arrayName)  // FlatArray[T]
	m.RegisterDefaultGaussdbType(&flatArrayOfT, arrayName) // *FlatArray[T]

	var flatArrayOfPtrT FlatArray[*T]
	m.RegisterDefaultGaussdbType(flatArrayOfPtrT, arrayName)  // FlatArray[*T]
	m.RegisterDefaultGaussdbType(&flatArrayOfPtrT, arrayName) // *FlatArray[*T]
}
