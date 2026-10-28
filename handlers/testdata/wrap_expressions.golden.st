// Any expression that does not fit is broken, not only call lists and IF conditions.
// A list is broken open first; an operator chain breaks before each of its operators,
// one level deeper than the line it starts on.

// Long arithmetic breaks at its lowest operators; the products stay whole.

PROGRAM arithmetic
totalEnergyConsumption := motorPowerPhaseOne * operatingHoursPhaseOne
	+ motorPowerPhaseTwo * operatingHoursPhaseTwo
	+ auxiliaryLoad;
END_PROGRAM

// A long assignment whose right side is a single call keeps the call on the first line.

PROGRAM assignment
resultingConfiguration := ConfigurationBuilder(
	sourceMachine := primaryMachineIdentifier,
	targetLine := productionLineIdentifier
);
END_PROGRAM

// Nested calls: each line the layout produces is laid out again, so a long argument breaks too.

PROGRAM nested
outerCall(
	firstArgument := innerCall(
		alphaParameter := alphaValueWithLongName,
		betaParameter := betaValueWithLongName,
		gammaParameter := gammaValue
	),
	second := 2
);
END_PROGRAM

// A call with one argument is not broken open; the call inside it is.

PROGRAM singleArgument
wrapperFunction(innerFunctionWithLongName(
	firstParameter := firstValueWithLongName,
	secondParameter := secondValueWithLongName
));
END_PROGRAM

// A chain with a call on either side breaks at the operator when breaking the call does not make it fit.

PROGRAM chainOfCalls
isWithinTolerance := ABS(measuredPositionOfTheAxis - commandedPositionOfTheAxis) < positionToleranceOfTheAxis
	AND axisIsEnabled;
END_PROGRAM

// A WHILE condition is laid out like an IF condition.

PROGRAM whileLoop
WHILE conveyorIsRunning
	AND NOT emergencyStopPressed
	AND productDetectedAtInfeed
	AND outfeedIsClear
	AND counter < maximumCount
DO
	counter := counter + 1;
END_WHILE
END_PROGRAM
