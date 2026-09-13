// Package pdf рендерит документ на выдачу в текстовый PDF на PT Sans.
//
// Шрифт вшит в бинарник (assets ниже): задание требует «текстовый, не картинка»
// и кириллический шрифт с Google Fonts, а рендер без интернета не должен
// зависеть от доступности CDN. Таблицу строк рисуем сами: специализированного
// табличного слоя в gopdf нет, а для восьми колонок сетка из Cell/Line проще
// внешней зависимости.
package pdf

import (
	_ "embed"

	"github.com/signintech/gopdf"
)

//go:embed fonts/PTSans-Regular.ttf
var regularFont []byte

//go:embed fonts/PTSans-Bold.ttf
var boldFont []byte

const (
	fontRegular = "PTSans"
	fontBold    = "PTSans-Bold"

	marginMM   = 18.0
	rowH       = 7.0
	lineGap    = 6.0
	cellPadX   = 2.5 // мм слева и справа: текст в ячейке не должен упираться в линии сетки
	textSize   = 10
	titleSize  = 13
	headerSize = 8 // header labels are longer than data, and the columns are narrow

	// Альбомный A4: восемь числовых колонок читаются без наложений.
	pageWidthMM  = 297.0
	pageHeightMM = 210.0
)

// columns — ширины столбцов таблицы строк в мм. Сумма с полями укладывается
// в альбомный A4: 297 − 2×18 = 261 мм.
var columns = []struct {
	title string
	width float64
	align int
}{
	{"Артикул", 22, gopdf.Left},
	{"Наименование", 86, gopdf.Left},
	{"Кол-во", 20, gopdf.Right},
	{"Цена", 26, gopdf.Right},
	{"Ставка", 20, gopdf.Center},
	{"Сумма", 29, gopdf.Right},
	{"НДС", 29, gopdf.Right},
	{"Сумма с НДС", 29, gopdf.Right},
}

// Renderer реализует usecase.DocumentRenderer.
type Renderer struct{}
