package exchange

import "testing"

func TestJoinVolumeCensus_Classes(t *testing.T) {
	t.Parallel()
	stored := []StoredVolume{
		{1, 10}, {2, 3}, {3, 99}, {4, 10},
	}
	auth := []VolumeAuthority{
		{OpenTime: 1, Base: 10, TakerBuyBase: 3, Quote: 1000, TakerBuyQuote: 400},
		{OpenTime: 2, Base: 10, TakerBuyBase: 3, Quote: 1000, TakerBuyQuote: 400},
		{OpenTime: 3, Base: 11, TakerBuyBase: 4, Quote: 99, TakerBuyQuote: 40},
		{OpenTime: 5, Base: 1, TakerBuyBase: 1, Quote: 1, TakerBuyQuote: 1},
	}
	c := JoinVolumeCensus(stored, auth)
	if c.Stored != 4 || c.Compared != 3 || c.NoAuthority != 1 || c.AuthorityExtra != 1 {
		t.Fatalf("%+v", c)
	}
	if c.TotalBase != 1 || c.TakerBuyBase != 1 || c.TotalQuote != 1 {
		t.Fatalf("classes %+v", c)
	}
}
