EAPI=8
DESCRIPTION="${PN} - A friendly greeting program"
HOMEPAGE="https://www.gnu.org/software/${PN}/"
SRC_URI="mirror://gnu/${PN}/${P}.tar.gz"

LICENSE="GPL-3"
SLOT="0"
KEYWORDS="*"
S="${WORKDIR}/${P}"

RDEPEND="sys-libs/zlib"
BDEPEND="sys-devel/gcc"