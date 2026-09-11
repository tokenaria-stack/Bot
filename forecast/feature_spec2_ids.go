package forecast

// FEATURE-SPEC-2 FeatureIDs. Order is identity.
const (
	FeaturesLogicV2 LogicVersion = "features:v2"
	PlanLogicV2     LogicVersion = "plan:v2"

	FeatureRSXMinusSignal FeatureID = "rsx_minus_signal"
	FeatureRSXDelta1      FeatureID = "rsx_delta_1"
	FeatureRSXDeltaQ      FeatureID = "rsx_delta_q"

	FeatureTVBearPresent FeatureID = "tv_bear_present"
	FeatureTVBearAge     FeatureID = "tv_bear_age"

	FeatureTVPivotHighPresent FeatureID = "tv_pivot_high_present"
	FeatureTVPivotHighAge     FeatureID = "tv_pivot_high_age"
	FeatureTVPivotLowPresent  FeatureID = "tv_pivot_low_present"
	FeatureTVPivotLowAge      FeatureID = "tv_pivot_low_age"

	FeatureRSXSignalCrossUpPresent   FeatureID = "rsx_signal_cross_up_present"
	FeatureRSXSignalCrossUpAge       FeatureID = "rsx_signal_cross_up_age"
	FeatureRSXSignalCrossDownPresent FeatureID = "rsx_signal_cross_down_present"
	FeatureRSXSignalCrossDownAge     FeatureID = "rsx_signal_cross_down_age"

	FeatureRSX50CrossUpPresent   FeatureID = "rsx_50_cross_up_present"
	FeatureRSX50CrossUpAge       FeatureID = "rsx_50_cross_up_age"
	FeatureRSX50CrossDownPresent FeatureID = "rsx_50_cross_down_present"
	FeatureRSX50CrossDownAge     FeatureID = "rsx_50_cross_down_age"

	FeaturePriceDisplacementQATR FeatureID = "price_displacement_q_atr"
	FeaturePriceDisplacementHATR FeatureID = "price_displacement_h_atr"
	FeaturePriceRangePositionH   FeatureID = "price_range_position_h"
	FeaturePricePathEfficiencyH  FeatureID = "price_path_efficiency_h"
	FeatureATROverPrice          FeatureID = "atr_over_price"
	FeatureATRChangeQ            FeatureID = "atr_change_q"

	FeaturePatTVToCrossBullPresent FeatureID = "pattern_tv_to_cross_bull_present"
	FeaturePatTVToCrossBullAge     FeatureID = "pattern_tv_to_cross_bull_age"
	FeaturePatTVToCrossBearPresent FeatureID = "pattern_tv_to_cross_bear_present"
	FeaturePatTVToCrossBearAge     FeatureID = "pattern_tv_to_cross_bear_age"

	FeaturePatCrossToTVBullPresent FeatureID = "pattern_cross_to_tv_bull_present"
	FeaturePatCrossToTVBullAge     FeatureID = "pattern_cross_to_tv_bull_age"
	FeaturePatCrossToTVBearPresent FeatureID = "pattern_cross_to_tv_bear_present"
	FeaturePatCrossToTVBearAge     FeatureID = "pattern_cross_to_tv_bear_age"

	FeaturePatPivotToCrossBullPresent FeatureID = "pattern_pivot_to_cross_bull_present"
	FeaturePatPivotToCrossBullAge     FeatureID = "pattern_pivot_to_cross_bull_age"
	FeaturePatPivotToCrossBearPresent FeatureID = "pattern_pivot_to_cross_bear_present"
	FeaturePatPivotToCrossBearAge     FeatureID = "pattern_pivot_to_cross_bear_age"

	FeaturePatSamebarTVCrossBullPresent FeatureID = "pattern_samebar_tv_cross_bull_present"
	FeaturePatSamebarTVCrossBullAge     FeatureID = "pattern_samebar_tv_cross_bull_age"
	FeaturePatSamebarTVCrossBearPresent FeatureID = "pattern_samebar_tv_cross_bear_present"
	FeaturePatSamebarTVCrossBearAge     FeatureID = "pattern_samebar_tv_cross_bear_age"

	FeatureHTF1hRSXValue         FeatureID = "htf_1h_rsx_value"
	FeatureHTF1hRSXMinusSignal   FeatureID = "htf_1h_rsx_minus_signal"
	FeatureHTF1hRSXDelta1        FeatureID = "htf_1h_rsx_delta_1"
	FeatureHTF1hTVBullPresent    FeatureID = "htf_1h_tv_bull_present"
	FeatureHTF1hTVBullAge        FeatureID = "htf_1h_tv_bull_age"
	FeatureHTF1hTVBearPresent    FeatureID = "htf_1h_tv_bear_present"
	FeatureHTF1hTVBearAge        FeatureID = "htf_1h_tv_bear_age"
	FeatureHTF1hSignalCrossUpP   FeatureID = "htf_1h_signal_cross_up_present"
	FeatureHTF1hSignalCrossUpAge FeatureID = "htf_1h_signal_cross_up_age"
	FeatureHTF1hSignalCrossDnP   FeatureID = "htf_1h_signal_cross_down_present"
	FeatureHTF1hSignalCrossDnAge FeatureID = "htf_1h_signal_cross_down_age"

	FeatureHTF4hRSXValue         FeatureID = "htf_4h_rsx_value"
	FeatureHTF4hRSXMinusSignal   FeatureID = "htf_4h_rsx_minus_signal"
	FeatureHTF4hRSXDelta1        FeatureID = "htf_4h_rsx_delta_1"
	FeatureHTF4hTVBullPresent    FeatureID = "htf_4h_tv_bull_present"
	FeatureHTF4hTVBullAge        FeatureID = "htf_4h_tv_bull_age"
	FeatureHTF4hTVBearPresent    FeatureID = "htf_4h_tv_bear_present"
	FeatureHTF4hTVBearAge        FeatureID = "htf_4h_tv_bear_age"
	FeatureHTF4hSignalCrossUpP   FeatureID = "htf_4h_signal_cross_up_present"
	FeatureHTF4hSignalCrossUpAge FeatureID = "htf_4h_signal_cross_up_age"
	FeatureHTF4hSignalCrossDnP   FeatureID = "htf_4h_signal_cross_down_present"
	FeatureHTF4hSignalCrossDnAge FeatureID = "htf_4h_signal_cross_down_age"
)

// FeatureSpec2IDs is the exact 64-column order.
func FeatureSpec2IDs() []FeatureID {
	return []FeatureID{
		FeatureRSXValue, FeatureRSXMinusSignal, FeatureRSXDelta1, FeatureRSXDeltaQ,
		FeatureTVBullPresent, FeatureTVBullAge, FeatureTVBearPresent, FeatureTVBearAge,
		FeatureTVPivotHighPresent, FeatureTVPivotHighAge, FeatureTVPivotLowPresent, FeatureTVPivotLowAge,
		FeatureRSXSignalCrossUpPresent, FeatureRSXSignalCrossUpAge, FeatureRSXSignalCrossDownPresent, FeatureRSXSignalCrossDownAge,
		FeatureRSX50CrossUpPresent, FeatureRSX50CrossUpAge, FeatureRSX50CrossDownPresent, FeatureRSX50CrossDownAge,
		FeaturePriceDisplacementQATR, FeaturePriceDisplacementHATR, FeaturePriceRangePositionH,
		FeaturePricePathEfficiencyH, FeatureATROverPrice, FeatureATRChangeQ,
		FeaturePatTVToCrossBullPresent, FeaturePatTVToCrossBullAge, FeaturePatTVToCrossBearPresent, FeaturePatTVToCrossBearAge,
		FeaturePatCrossToTVBullPresent, FeaturePatCrossToTVBullAge, FeaturePatCrossToTVBearPresent, FeaturePatCrossToTVBearAge,
		FeaturePatPivotToCrossBullPresent, FeaturePatPivotToCrossBullAge, FeaturePatPivotToCrossBearPresent, FeaturePatPivotToCrossBearAge,
		FeaturePatSamebarTVCrossBullPresent, FeaturePatSamebarTVCrossBullAge, FeaturePatSamebarTVCrossBearPresent, FeaturePatSamebarTVCrossBearAge,
		FeatureHTF1hRSXValue, FeatureHTF1hRSXMinusSignal, FeatureHTF1hRSXDelta1,
		FeatureHTF1hTVBullPresent, FeatureHTF1hTVBullAge, FeatureHTF1hTVBearPresent, FeatureHTF1hTVBearAge,
		FeatureHTF1hSignalCrossUpP, FeatureHTF1hSignalCrossUpAge, FeatureHTF1hSignalCrossDnP, FeatureHTF1hSignalCrossDnAge,
		FeatureHTF4hRSXValue, FeatureHTF4hRSXMinusSignal, FeatureHTF4hRSXDelta1,
		FeatureHTF4hTVBullPresent, FeatureHTF4hTVBullAge, FeatureHTF4hTVBearPresent, FeatureHTF4hTVBearAge,
		FeatureHTF4hSignalCrossUpP, FeatureHTF4hSignalCrossUpAge, FeatureHTF4hSignalCrossDnP, FeatureHTF4hSignalCrossDnAge,
	}
}
