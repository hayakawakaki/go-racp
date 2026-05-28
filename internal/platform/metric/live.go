package metric

var live *Reader

func SetLive(r *Reader) { live = r }

func Live() *Reader { return live }
