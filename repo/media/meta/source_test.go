package meta

import "testing"

type loggingSource struct {
	t *testing.T
}

func (ls *loggingSource) FromFile(path string) (Metadata, error) {
	ls.t.Logf("file %s", path)
	return nil, nil
}

func (ls *loggingSource) FromQuery(query Query) (Metadata, error) {
	ls.t.Logf("query %s, type %d, season %d, episode %d", query.Query(), query.Type(), query.Season(), query.Episode())
	return nil, nil
}

func TestFileAnalysisSource_FromFile(t *testing.T) {
	metaSource := NewFileAnalysisSource(&loggingSource{t: t})
	metaSource.FromFile("Noragami Aragoto 13 CZ.mkv")
	metaSource.FromFile("Bocchi the Rock! 12 (CZ, 720p).mkv")
	metaSource.FromFile("Chicago.Med.S01E10 cz.tit..avi")
	metaSource.FromFile("Nemocnice Chicago Med s01x09 CZdab.avi")
	metaSource.FromFile("chicago.med.s06e09.720p.hdtv.x264-syncopy[eztv.re].mkv")
	metaSource.FromFile("Babovřesky 3 (2015) [juraison+].avi")
}
