(* Motion axis wrapper: profile generator, homing and soft limits. *)

FUNCTION_BLOCK FB_Axis
VAR_INPUT
    bEnable         : BOOL;
    fVelocity       : LREAL;
    fAcceleration   : LREAL;
END_VAR

VAR_OUTPUT
    bBusy           : BOOL;
    bDone           : BOOL;
    fPosition       : LREAL;
    nError          : INT;
END_VAR

VAR
    mcMove          : MC_MoveAbsolute;
    mcHalt          : MC_Halt;
    mcHome          : MC_Home;
    bHomed          : BOOL;
    fSoftMin        : LREAL := -1000.0;
    fSoftMax        : LREAL := 1000.0;
END_VAR

    // Soft limits are checked before the command is handed to the axis.
    IF fVelocity > 0.0 AND fPosition + (fVelocity * 0.5) > fSoftMax THEN
        nError := 101;
        bBusy := FALSE;
        bDone := TRUE;
        RETURN;
    END_IF

    IF fVelocity < 0.0 AND fPosition + (fVelocity * 0.5) < fSoftMin THEN
        nError := 102;
        bBusy := FALSE;
        bDone := TRUE;
        RETURN;
    END_IF

    IF bEnable AND NOT bHomed THEN
        mcHome(
            bExecute := TRUE,
            bClearError := TRUE,
            fHomePosition := 0.0,
            fSpeed := 50.0,
            fAccel := 100.0,
            tTimeout := T#30S
        );
        IF mcHome.bDone THEN
            bHomed := TRUE;
            nError := 0;
        END_IF
        IF mcHome.bError THEN
            nError := mcHome.nErrorID;
        END_IF
        bBusy := mcHome.bBusy;
        bDone := mcHome.bDone;
        RETURN;
    END_IF

    IF bEnable THEN
        mcMove(
            bExecute := TRUE,
            fTargetPosition := fVelocity,
            fVelocity := fVelocity,
            fAccel := fAcceleration,
            fDecel := fAcceleration,
            bAbsolute := TRUE,
            tTimeout := T#5S
        );
        bBusy := mcMove.bBusy;
        bDone := mcMove.bDone;
        fPosition := mcMove.fActualPosition;
        IF mcMove.bError THEN
            nError := mcMove.nErrorID;
        END_IF
        RETURN;
    END_IF

    mcHalt(bExecute := TRUE, bDecel := TRUE, fDecel := fAcceleration);
    bBusy := mcHalt.bBusy;
    bDone := mcHalt.bDone;
    fPosition := mcHalt.fActualPosition;
END_FUNCTION_BLOCK

FUNCTION F_Clamp : LREAL
VAR_INPUT
    fValue          : LREAL;
    fMinimum        : LREAL;
    fMaximum        : LREAL;
END_VAR

    IF fValue < fMinimum THEN
        F_Clamp := fMinimum;
    ELSIF fValue > fMaximum THEN
        F_Clamp := fMaximum;
    ELSE
        F_Clamp := fValue;
    END_IF
END_FUNCTION

FUNCTION F_DegToRad : LREAL
VAR_INPUT
    fDegrees        : LREAL;
END_VAR

    F_DegToRad := fDegrees * (PI() / 180.0);
END_FUNCTION