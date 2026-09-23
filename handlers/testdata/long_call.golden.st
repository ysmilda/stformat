PROGRAM p
cmd.FB_Build(
	rDoExtending := rDoExtending,
	rDoRetracting := rDoRetracting,
	rDiAtExtendedPosition := rDiAtExtendedPosition,
	rDiAtRetractedPosition := rDiAtRetractedPosition,
	commandTimeout := T#5S,
	driveEnable := TRUE,
	speedFactor := 1.0
);
cmd.FB_Build(a := 1, b := 2);
END_PROGRAM
