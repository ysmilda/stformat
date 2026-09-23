PROGRAM WrapIf

VAR
    flagOperationalReady    : BOOL;
    flagMaintenanceOverride : BOOL;
    flagEmergencyStopActive : BOOL;
    flagPowerAvailable      : BOOL;
    flagGateClosed          : BOOL;
    flagInterlockEngaged    : BOOL;
    flagSequenceRunning     : BOOL;
    flagAbortRequested      : BOOL;
    flagResetPermitted      : BOOL;
    flagCycleComplete       : BOOL;
    flagAutoModeSelected    : BOOL;
    flagManualModeSelected  : BOOL;
    flagRemoteModeSelected  : BOOL;
    flagLocalModeSelected   : BOOL;
    result                  : BOOL;
    cmdText                 : STRING(80);
END_VAR

// Simple long IF: split at top-level AND.
IF flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected THEN
    result := TRUE;
END_IF

// Parenthesised combined condition stays together when it fits.
IF (flagOperationalReady OR flagMaintenanceOverride) AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning THEN
    result := TRUE;
END_IF

// Over-long combined condition is deep-split at its own AND operators.
IF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested) AND flagResetPermitted THEN
    result := TRUE;
END_IF

// ELSIF also wraps.
IF flagOperationalReady THEN
    result := FALSE;
ELSIF flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN
    result := TRUE;
END_IF

// Short IF stays inline.
IF flagOperationalReady AND flagMaintenanceOverride THEN
    result := TRUE;
END_IF

// XOR chain wraps the same way.
IF flagOperationalReady OR flagMaintenanceOverride OR flagEmergencyStopActive OR flagPowerAvailable OR flagGateClosed OR flagInterlockEngaged THEN
    result := TRUE;
END_IF

// String literal containing THEN/AND is not split.
cmdText := 'FIND THEN AND OR XOR';
IF cmdText = 'FIND THEN AND OR XOR' AND flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed THEN
    result := TRUE;
END_IF

// Mixed AND/OR with two short paren groups.
IF (flagOperationalReady AND flagMaintenanceOverride) OR (flagEmergencyStopActive AND flagPowerAvailable) AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN
    result := TRUE;
END_IF

// Over-long first paren group deep-splits; second short group stays whole.
IF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagAbortRequested AND flagResetPermitted) THEN
    result := TRUE;
END_IF

// Both paren groups overlong: deep-split at two indent levels.
IF (flagOperationalReady AND flagMaintenanceOverride AND flagEmergencyStopActive AND flagPowerAvailable AND flagGateClosed AND flagInterlockEngaged) OR (flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete AND flagAutoModeSelected AND flagManualModeSelected) THEN
    result := TRUE;
END_IF

// Nested parens inside parens: inner AND/OR stays with its group.
IF ((flagOperationalReady OR flagMaintenanceOverride) AND (flagEmergencyStopActive OR flagPowerAvailable)) AND flagGateClosed AND flagInterlockEngaged AND flagSequenceRunning AND flagAbortRequested THEN
    result := TRUE;
END_IF

// Mixed top-level: multiple paren groups separated by OR, with trailing AND chain.
IF (flagOperationalReady AND flagMaintenanceOverride) OR (flagEmergencyStopActive AND flagPowerAvailable) OR (flagGateClosed AND flagInterlockEngaged) AND flagSequenceRunning AND flagAbortRequested AND flagResetPermitted AND flagCycleComplete THEN
    result := TRUE;
END_IF

END_PROGRAM