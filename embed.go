package copya

import "embed"

//go:embed seeds/common/*.csv
//go:embed seeds/general/*.csv
//go:embed seeds/service/*.csv
//go:embed seeds/professional/*.csv
var SeedsFS embed.FS
