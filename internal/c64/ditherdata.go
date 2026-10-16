// Data measured with tools/ditheranalysis.py on 16 C64 multicolor pictures.

package c64

// The tables below were measured on 16 hand-pixelled C64 multicolor pictures
// (VICE screenshots, Pepto palette): 6381 two-colored 8×8 dither areas and
// 33000 4×4 windows. See tools/ditheranalysis.py for the method.
//
// Findings:
//   - Artists mix colors of neighboring brightness: 54 % of all mixed pairs are
//     one luma step apart, 29 % two steps, 11 % three, almost never more.
//   - Most dithers live on every second row ("interlaced"): alternate rows
//     filled first, then the rows in between.
//   - Groups of pictures use distinct pattern families: scanlines, horizontal
//     dashes (hatching), checkerboards, vertical columns and scattered dots.

// styleMatrix holds the fill order (0 = first pixel) of the ordered styles.
// Each was fitted greedily to the most frequent patterns of its picture group.
var styleMatrix = map[DitherStyle][]int{
	// all 16 pictures: checker on alternate rows, then full rows, then the rows in between
	StyleArtist: {0, 4, 2, 6, 8, 12, 10, 14, 5, 1, 7, 3, 13, 9, 15, 11},
	// US Gold portrait, diamond, glitch girl, Beavis, Bomb Jack: alternate scanlines
	StyleScanlines: {0, 2, 6, 4, 8, 12, 10, 14, 1, 3, 5, 7, 13, 9, 15, 11},
	// ghost girl, old woman, dragon: classic checkerboard (Bayer order)
	StyleChecker: {0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5},
	// birthday biker, abstract faces, Gameland: vertical lines that widen into stripes
	StyleStripes: {0, 6, 8, 14, 2, 4, 10, 12, 1, 7, 9, 15, 3, 5, 11, 13},
	StyleColor:   {0, 4, 2, 6, 8, 12, 10, 14, 5, 1, 7, 3, 13, 9, 15, 11}, // shading uses the artists' pattern
}

// pairCountDither[a][b] counts 8×8 dither areas mixing colors a and b.
var pairCountDither = [16][16]uint16{
	{0, 55, 106, 11, 2, 0, 1877, 1, 487, 514, 1, 78, 1, 0, 1, 0},
	{55, 0, 0, 14, 0, 0, 0, 497, 0, 0, 128, 0, 0, 344, 0, 83},
	{106, 0, 0, 0, 11, 0, 96, 4, 108, 0, 7, 0, 0, 0, 7, 0},
	{11, 14, 0, 0, 0, 4, 0, 5, 0, 0, 22, 0, 44, 121, 25, 0},
	{2, 0, 11, 0, 0, 2, 286, 0, 1, 0, 89, 0, 0, 0, 20, 0},
	{0, 0, 0, 4, 2, 0, 0, 7, 0, 0, 0, 0, 2, 0, 1, 1},
	{1877, 0, 96, 0, 286, 0, 0, 0, 0, 41, 0, 10, 34, 0, 12, 0},
	{1, 497, 4, 5, 0, 7, 0, 0, 0, 0, 103, 0, 0, 0, 6, 15},
	{487, 0, 108, 0, 1, 0, 0, 0, 0, 2, 343, 34, 0, 0, 5, 0},
	{514, 0, 0, 0, 0, 0, 41, 0, 2, 0, 0, 0, 0, 0, 0, 0},
	{1, 128, 7, 22, 89, 0, 0, 103, 343, 0, 0, 0, 38, 55, 6, 129},
	{78, 0, 0, 0, 0, 0, 10, 0, 34, 0, 0, 0, 137, 0, 0, 0},
	{1, 0, 0, 44, 0, 2, 34, 0, 0, 0, 38, 137, 0, 0, 0, 252},
	{0, 344, 0, 121, 0, 0, 0, 0, 0, 0, 55, 0, 0, 0, 0, 96},
	{1, 0, 7, 25, 20, 1, 12, 6, 5, 0, 6, 0, 0, 0, 0, 0},
	{0, 83, 0, 0, 0, 1, 0, 15, 0, 0, 129, 0, 252, 96, 0, 0},
}

// pairCountAll[a][b] counts all 4×4 windows that contain exactly colors a and b.
var pairCountAll = [16][16]uint16{
	{0, 355, 1374, 94, 317, 96, 7143, 106, 1271, 2588, 206, 1135, 193, 40, 168, 57},
	{355, 0, 2, 210, 12, 17, 25, 1664, 5, 1, 778, 17, 38, 1266, 2, 454},
	{1374, 2, 0, 6, 171, 11, 298, 45, 419, 91, 392, 3, 14, 0, 46, 18},
	{94, 210, 6, 0, 39, 114, 28, 207, 4, 1, 184, 1, 220, 540, 305, 11},
	{317, 12, 171, 39, 0, 65, 1420, 10, 27, 10, 421, 12, 10, 0, 207, 3},
	{96, 17, 11, 114, 65, 0, 28, 88, 13, 12, 38, 2, 54, 56, 111, 56},
	{7143, 25, 298, 28, 1420, 28, 0, 23, 22, 97, 39, 174, 146, 4, 425, 5},
	{106, 1664, 45, 207, 10, 88, 23, 0, 35, 2, 697, 1, 18, 36, 86, 339},
	{1271, 5, 419, 4, 27, 13, 22, 35, 0, 217, 1219, 171, 79, 0, 58, 25},
	{2588, 1, 91, 1, 10, 12, 97, 2, 217, 0, 19, 65, 48, 0, 7, 2},
	{206, 778, 392, 184, 421, 38, 39, 697, 1219, 19, 0, 10, 299, 153, 128, 858},
	{1135, 17, 3, 1, 12, 2, 174, 1, 171, 65, 10, 0, 630, 1, 34, 1},
	{193, 38, 14, 220, 10, 54, 146, 18, 79, 48, 299, 630, 0, 9, 4, 754},
	{40, 1266, 0, 540, 0, 56, 4, 36, 0, 0, 153, 1, 9, 0, 13, 419},
	{168, 2, 46, 305, 207, 111, 425, 86, 58, 7, 128, 34, 4, 13, 0, 33},
	{57, 454, 18, 11, 3, 56, 5, 339, 25, 2, 858, 1, 754, 419, 33, 0},
}
