PROGRAM p
VAR_INPUT
	a              : BOOL;
	start_position : REAL := 1.0;
	count          : INT;
END_VAR

VAR_OUTPUT
	result : STRING(32);
END_VAR

VAR_TEMP
	tempbuf : ARRAY[1..10] OF INT;
END_VAR

VAR_GLOBAL RETAIN
	max_speed : UDINT := 2000;
END_VAR

VAR
	pin_ptr : POINTER TO INT;
	ref_var : REFERENCE TO BOOL;
	mapped  : INT AT %mb30;
END_VAR
END_PROGRAM
