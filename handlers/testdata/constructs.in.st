// Namespaces, configurations, property accessors and SFC, whose openers are
// recognised only where their closing keyword follows.
NAMESPACE Motion
FUNCTION F_A : INT
F_A := 1;
END_FUNCTION
FUNCTION F_B : INT
F_B := 2;
END_FUNCTION
END_NAMESPACE

CONFIGURATION Cell
VAR_GLOBAL bRun : BOOL; END_VAR
RESOURCE Cpu ON PLC
TASK Fast(INTERVAL := T#10MS, PRIORITY := 1);
PROGRAM Main WITH Fast : P_Main;
END_RESOURCE
END_CONFIGURATION

FUNCTION_BLOCK FB_Prop
PROPERTY Value : INT
GET
Value := nValue;
END_GET
SET
nValue := Value;
END_SET
END_PROPERTY
END_FUNCTION_BLOCK

PROGRAM Chart
INITIAL_STEP Idle: Reset(N);
END_STEP
STEP Run: Drive(N);
END_STEP
TRANSITION FROM Idle TO Run := bStart;
END_TRANSITION
END_PROGRAM

// The same words used as names stay names.
PROGRAM Names
VAR Step, Set, Get : INT; END_VAR
Step := Step + 1;
Set := Get;
fb.Set(1);
IF Step > 3 THEN Step := 0; END_IF
END_PROGRAM

// An opener right after a POU header, with no VAR block between.
PROGRAM Direct
STEP Run:
DriveTheAxisToPosition(Position := fTargetPositionInMillimetres, Velocity := fVelocityLimit, Acceleration := fAcc);
END_STEP
END_PROGRAM
