FUNCTION max2 : INT
max2 := a;
END_FUNCTION

FUNCTION_BLOCK motor
VAR_INPUT
	cmd   : BOOL;
	speed : REAL;
END_VAR

METHOD Execute : BOOL
VAR
	result : BOOL;
END_VAR

result := cmd;
END_METHOD
ACTION do_it
speed := 10.0;
END_ACTION
END_FUNCTION_BLOCK

FUNCTION combine : INT
combine := max2(1, 2);
END_FUNCTION
