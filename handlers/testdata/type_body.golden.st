// The value list of a TYPE.
// A TYPE body is a list of values like a call's argument list, so it breaks at the same limit into the same layout.
// The bracket depth is carried from line to line, and text that is not structure must not feed it.

// A body over the limit breaks one member per line.

TYPE EState :
	(
		A := 0,
		B := 1,
		C := 2,
		D := 3,
		E := 4,
		F := 5,
		G := 6,
		H := 7,
		I := 8,
		J := 9,
		K := 10,
		L := 11,
		M := 12,
		N := 13,
		O := 14,
		P := 15
	);
END_TYPE

// One member cannot be split any further, so the line stays long.

TYPE t :
	(
		A := 1000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000
	);
END_TYPE

// A parenthesis inside a comment is text, so it does not open a level.
(* note ( unbalanced *)

TYPE FromComment :
	(
		A := 0,
		B := 1,
		C := 2,
		D := 3,
		E := 4,
		F := 5,
		G := 6,
		H := 7,
		I := 8,
		J := 9,
		K := 10,
		L := 11,
		M := 12,
		N := 13,
		O := 14,
		P := 15
	);
END_TYPE

// Same for an ignored region: the count restarts at stformat:on.

TYPE FromRegion :
	// stformat:off
weird ( unbalanced ( ( (
	// stformat:on
	(
		A := 0,
		B := 1,
		C := 2,
		D := 3,
		E := 4,
		F := 5,
		G := 6,
		H := 7,
		I := 8,
		J := 9,
		K := 10,
		L := 11,
		M := 12,
		N := 13,
		O := 14,
		P := 15
	);
END_TYPE
