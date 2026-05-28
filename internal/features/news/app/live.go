package app

var live *Service

func SetLive(s *Service) { live = s }

func Live() *Service { return live }
