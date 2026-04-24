test-hiragana:
	@echo 'Hiragana:'
	go run . -fzf=false -r ぱん && echo ''
	go run . -fzf=false -r ごはん && echo ''
	go run . -fzf=false -r かんじる && echo ''

test-katakana:
	@echo 'Katakana:'
	go run . -fzf=false -r パン && echo ''
	go run . -fzf=false -r パソコン && echo ''
	go run . -fzf=false -r スーパーマーケット && echo ''

test-romaji:
	@echo 'Romaji:'
	go run . -fzf=false -r sonzai && echo ''
	go run . -fzf=false -r aruji && echo ''
	go run . -fzf=false -r gohan && echo ''
	go run . -fzf=false -r tomodachi && echo ''

test-english:
	@echo 'English:'
	go run . -fzf=false -r -e bread && echo ''
	go run . -fzf=false -r -e existence && echo ''
	go run . -fzf=false -r -e apple && echo ''
	go run . -fzf=false -r -e light && echo ''
	go run . -fzf=false -r -e computer && echo ''

test-by-id:
	@echo 'By id:'
	go run . -fzf=false -id 115754 && echo '' # sonzai
	go run . -fzf=false -id 84386 && echo '' # ??

test-kanji-hiragana:
	@echo 'Kanji + Hiragana:'
	go run . -fzf=false -r 食べる && echo ''
	go run . -fzf=false -r 感じる && echo ''
	go run . -fzf=false -r 行く
	go run . -fzf=false -r 見る
	go run . -fzf=false -r 話す
	go run . -fzf=false -r 考える

test-translations:
	@echo 'Translations:'
	go run . -fzf=false -e panic

test-all:
	make test-hiragana
	make test-katakana
	make test-romaji
	make test-english
	make test-by-id
	make test-kanji-hiragana
