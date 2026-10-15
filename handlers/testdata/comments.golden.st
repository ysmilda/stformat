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
// A comment standing on its own line inside a condition.
// The tokens around it ask for no line break, so a second pass used to glue it to the line above.

PROGRAM p
IF a
// Both must be set
AND b THEN
	c := 1;
END_IF
END_PROGRAM
// A trailing comment on a condition that has to wrap moves to the THEN line.

PROGRAM p
IF flagOperationalReady
	AND flagMaintenanceOverride
	AND flagEmergencyStopActive
	AND flagPowerAvailable
	AND flagGateClosed
	AND flagInterlockEngaged
	AND flagSequenceRunning
	AND flagAbortRequested
	AND flagResetPermitted
	AND flagCycleComplete
	AND flagAutoModeSelected
THEN  // All flags
	x := TRUE;
END_IF
END_PROGRAM

FUNCTION_BLOCK fb EXTENDS base
// Built by the comment test
VAR
	y : INT;
END_VAR

y := 1;
END_FUNCTION_BLOCK
// A type with the comment on the line that opens its body

TYPE states :
	(
	// The state names
	a := 0, b := 1);
END_TYPE

FUNCTION_BLOCK child
PUBLIC EXTENDS base
// Inherits from base
VAR
	w : INT;
END_VAR
END_FUNCTION_BLOCK
