PROGRAM p
VAR
	x : INT;  // The counter
END_VAR

x := 1;  // Set x
(* Block
comment *)
z := 2;
IF x THEN
	y := x;  // Branch
END_IF  // Done
(* Note *)
z := 3;
END_PROGRAM
