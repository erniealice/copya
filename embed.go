package copya

import "embed"

//go:embed seeds/common/*.csv
//go:embed seeds/general/*.csv
//go:embed seeds/service/*.csv
//go:embed seeds/professional/*.csv
//go:embed seeds/jurisdictions/ph/*.csv
var SeedsFS embed.FS
