{attribute 'qualified_only'}
{attribute 'hide'}
TYPE E_LineState :
(
    eStateIdle := 0,
    eStateRunning := 10,
    eStateHoming := 20,
    eStateFault := 99
);
END_TYPE

TYPE E_Result :
(
    eResultOk := 0,
    eResultFault := 1,
    eResultTimeout := 2,
    eResultBusy := 3
);
END_TYPE

TYPE ST_LineStatus :
STRUCT
    iProduced       : INT;
    iRejected       : INT;
    fAvailability   : LREAL;
    sFault          : STRING(80);
    tLastChange     : TIME;
    arrStationBusy  : ARRAY[1..3] OF BOOL;
END_STRUCT
END_TYPE

TYPE ST_AxisConfig :
STRUCT
    fVelocity       : LREAL;
    fAcceleration   : LREAL;
    fDeceleration   : LREAL;
    tTimeout        : TIME;
END_STRUCT
END_TYPE

TYPE ST_Diagnostics :
UNION
    sText           : STRING(32);
    iCode           : INT;
    arrWords        : ARRAY[0..3] OF WORD;
END_UNION
END_TYPE

TYPE E_IOQuality : INT (0 .. 7); END_TYPE

TYPE stRecipeEntry :
STRUCT
    sName           : STRING(32);
    fSetpoint       : LREAL;
    iFlags          : WORD;
END_STRUCT
END_TYPE

VAR_GLOBAL CONSTANT
    cAXIS_COUNT     : INT := 4;
    cDEFAULT_SPEED  : LREAL := 250.0;
    cPIPE_DIAMETER  : LREAL := 40.0;
    cMAX_CYCLE_MS   : UDINT := 250;
    cRecipeSlots    : INT := 16;
END_VAR

VAR_GLOBAL
    gAxis           : ARRAY[1..cAXIS_COUNT] OF FB_Axis;
    gRecipe         : ARRAY[1..cRecipeSlots] OF ST_RecipeEntry;
    gCycleCounter   : UDINT;
    gFirstCycleDone : BOOL;
END_VAR