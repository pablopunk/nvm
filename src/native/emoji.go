package main

import (
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const emojiData = `😀 grinning face smile happy
😃 smiley big eyes
😄 smile laugh
😁 beaming grin
😆 laughing squint
😅 sweat smile relief
🤣 rofl rolling laughing
😂 joy tears laugh
🙂 slightly smiling
🙃 upside down
😉 wink
😊 blush smiling eyes
😇 angel innocent halo
🥰 smiling hearts love
😍 heart eyes love
🤩 star struck wow
😘 kiss blow
😋 yum delicious tongue
😛 tongue
😜 wink tongue crazy
🤪 zany crazy
🤗 hug hugging
🤔 thinking hmm
🤨 raised eyebrow skeptical
😐 neutral
😑 expressionless
😶 no mouth silent
🙄 eye roll
😏 smirk
😒 unamused
😞 disappointed sad
😔 pensive
😟 worried
😕 confused
😢 cry sad tear
😭 sob crying loud
😤 triumph huff
😠 angry mad
😡 rage pouting red
🤬 cursing swearing
🤯 mind blown exploding head
😳 flushed embarrassed
🥵 hot sweating
🥶 cold freezing
😱 scream fear shocked
😨 fearful
😰 anxious sweat
😥 sad relieved
😓 downcast sweat
🤭 hand over mouth giggle
🤫 shush quiet secret
🤥 lying liar pinocchio
😴 sleeping zzz
🤤 drooling
😷 mask sick
🤒 thermometer ill
🤕 bandage hurt
🤢 nauseated sick
🤮 vomit
🤧 sneeze
😎 cool sunglasses
🤓 nerd glasses
🧐 monocle curious
🥳 party celebrate
🥺 pleading puppy eyes
🫠 melting
🫡 salute
🫶 heart hands
👍 thumbs up like yes approve
👎 thumbs down dislike no
👌 ok perfect
✌️ victory peace
🤞 fingers crossed luck
🤟 love you gesture
🤘 rock on horns
👈 point left
👉 point right
👆 point up
👇 point down
☝️ index up
✋ raised hand stop
👋 wave hello bye
🤝 handshake deal
🙏 pray thanks please folded hands
👏 clap applause
🙌 raising hands hooray
💪 muscle strong flex
🦾 mechanical arm
🖕 middle finger
✍️ writing hand
👀 eyes look
👁️ eye
👂 ear
👃 nose
👄 mouth lips
🧠 brain smart
💀 skull dead
👻 ghost boo
👽 alien
🤖 robot
💩 poop
🙈 see no evil monkey
🙉 hear no evil monkey
🙊 speak no evil monkey
❤️ red heart love
🧡 orange heart
💛 yellow heart
💚 green heart
💙 blue heart
💜 purple heart
🖤 black heart
🤍 white heart
💔 broken heart
💕 two hearts
💖 sparkling heart
💯 hundred perfect score
💥 boom collision
💫 dizzy star
💦 sweat drops water
💨 dash wind
🔥 fire hot lit flame
✨ sparkles magic new
⭐ star
🌟 glowing star
⚡ lightning zap bolt
🌈 rainbow
☀️ sun sunny
🌙 moon crescent night
☁️ cloud
🌧️ rain
⛈️ storm thunder
❄️ snowflake snow cold
🌊 wave ocean water
🌍 earth globe world
🌸 cherry blossom flower
🌹 rose flower
🌻 sunflower
🌲 evergreen tree
🌴 palm tree
🍀 clover luck
🍄 mushroom
🐶 dog puppy
🐱 cat kitty
🐭 mouse
🐹 hamster
🐰 rabbit bunny
🦊 fox
🐻 bear
🐼 panda
🐨 koala
🐯 tiger
🦁 lion
🐮 cow
🐷 pig
🐸 frog
🐵 monkey
🐔 chicken
🐧 penguin
🐦 bird
🦆 duck
🦉 owl
🦄 unicorn
🐝 bee honey
🦋 butterfly
🐢 turtle
🐍 snake
🐙 octopus
🐬 dolphin
🐳 whale
🦈 shark
🍎 apple red
🍊 orange tangerine
🍋 lemon
🍌 banana
🍉 watermelon
🍇 grapes
🍓 strawberry
🍑 peach
🥑 avocado
🍅 tomato
🥕 carrot
🌽 corn
🍕 pizza
🍔 burger hamburger
🍟 fries
🌭 hot dog
🌮 taco
🍣 sushi
🍜 ramen noodles
🍝 spaghetti pasta
🍞 bread
🧀 cheese
🥚 egg
🍳 cooking fried egg
🍰 cake
🎂 birthday cake
🍪 cookie
🍫 chocolate
🍩 donut
🍦 ice cream
☕ coffee hot drink
🍵 tea
🍺 beer
🍻 cheers beers
🍷 wine
🥂 champagne toast
🥤 cup straw soda
⚽ soccer football
🏀 basketball
🏈 american football
🎾 tennis
🎮 video game controller
🎲 dice game
🎯 dart target bullseye
🎵 music note
🎸 guitar
🎤 microphone sing
🎧 headphones
🎬 clapper movie
🎨 art palette paint
📚 books
📖 open book
📝 memo note write
📎 paperclip attach
📌 pushpin pin
✂️ scissors cut
🔒 lock locked secure
🔓 unlock
🔑 key
🔨 hammer
🔧 wrench tool
⚙️ gear settings
🔗 link chain
📦 package box
📧 email mail
📱 phone mobile
💻 laptop computer
🖥️ desktop computer
⌨️ keyboard
🖱️ mouse computer
💾 floppy save
📷 camera photo
📹 video camera
📺 tv television
⏰ alarm clock
⌛ hourglass time
📅 calendar date
📈 chart up increase
📉 chart down decrease
📊 bar chart
💡 light bulb idea
🔍 magnifying glass search
🔔 bell notification
🔕 bell muted
🚀 rocket launch ship
✈️ airplane flight
🚗 car
🚲 bicycle bike
🚂 train
🚢 ship boat
🏠 house home
🏢 office building
🏥 hospital
🏖️ beach
⛰️ mountain
🗽 statue of liberty
🎉 party popper tada celebration
🎊 confetti
🎁 gift present
🏆 trophy winner
🥇 gold medal first
🏁 checkered flag finish
✅ check mark done yes
❌ cross mark no wrong
⚠️ warning caution
❗ exclamation
❓ question
🚫 prohibited no
➕ plus
➖ minus
➡️ right arrow
⬅️ left arrow
⬆️ up arrow
⬇️ down arrow
🔄 refresh reload counterclockwise
🔙 back arrow
🆕 new
🆗 ok button
💬 speech bubble comment chat
💭 thought bubble
👑 crown king
💎 gem diamond
💰 money bag
💸 money flying
🪄 magic wand
🛠️ tools hammer wrench
🧪 test tube experiment
🔬 microscope
🧬 dna
🪲 bug beetle
🐛 bug caterpillar
→ arrow right symbol
← arrow left symbol
↑ arrow up symbol
↓ arrow down symbol
↔ arrow left right symbol
⇒ double arrow implies symbol
± plus minus symbol math
× multiplication times symbol
÷ division symbol
≠ not equal symbol
≈ approximately symbol
≤ less or equal symbol
≥ greater or equal symbol
∞ infinity symbol
√ square root symbol
π pi symbol
Σ sigma sum symbol
∆ delta triangle symbol
° degree symbol
€ euro currency symbol
£ pound currency symbol
¥ yen currency symbol
¢ cent currency symbol
₿ bitcoin currency symbol
© copyright symbol
® registered symbol
™ trademark symbol
§ section symbol
¶ paragraph pilcrow symbol
• bullet symbol
… ellipsis symbol
✓ check tick symbol
✗ cross ballot symbol
★ star filled symbol
☆ star outline symbol
♥ heart suit symbol
½ one half fraction symbol
¼ quarter fraction symbol
² squared superscript symbol
³ cubed superscript symbol
⌘ command key mac symbol
⌥ option alt key symbol
⇧ shift key symbol
⌃ control key symbol
⏎ return enter key symbol
⌫ delete backspace key symbol`

type emojiEntry struct{ Glyph, Name string }

type emojiState struct {
	all      []emojiEntry
	filtered []emojiEntry
	lastQ    string
	chosen   int
	list     ui.ListState
	row      int
}

const emojiCols = 12

func (s *emojiState) load() {
	for _, l := range strings.Split(emojiData, "\n") {
		g, name, ok := strings.Cut(l, " ")
		if ok {
			s.all = append(s.all, emojiEntry{g, name})
		}
	}
}

func (s *emojiState) refilter(q string) {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == s.lastQ && s.filtered != nil {
		return
	}
	s.lastQ, s.chosen = q, 0
	s.filtered = s.filtered[:0]
	for _, e := range s.all {
		if q == "" || strings.Contains(e.Name, q) {
			s.filtered = append(s.filtered, e)
		}
	}
}

func (p *Palette) emojiView() *View {
	s := &p.emoji
	s.lastQ, s.filtered = "\x00", nil
	pick := func(paste bool) {
		if s.chosen < len(s.filtered) {
			g := s.filtered[s.chosen].Glyph
			p.Hide()
			if paste {
				pasteText(g)
			} else {
				mygo.Clipboard.WriteText(g)
			}
		}
	}
	return &View{
		Title: "Emoji & Symbols", Placeholder: "Search Emoji & Symbols",
		Hint: "↵ Copy   ⌃↵ Paste   ←↑↓→ Move", KeepQuery: true,
		OnSubmit: func(string) { pick(false) },
		Status: func() string {
			if s.chosen < len(s.filtered) {
				e := s.filtered[s.chosen]
				return e.Glyph + "  " + strings.Fields(e.Name + " ")[0]
			}
			return "no match"
		},
		OnKey: func(c *ui.Context, p *Palette) bool {
			switch {
			case c.Shortcut(0, ui.KeyRight):
				s.chosen = min(s.chosen+1, len(s.filtered)-1)
			case c.Shortcut(0, ui.KeyLeft):
				s.chosen = max(s.chosen-1, 0)
			case c.Shortcut(0, ui.KeyDown):
				s.chosen = min(s.chosen+emojiCols, len(s.filtered)-1)
			case c.Shortcut(0, ui.KeyUp):
				s.chosen = max(s.chosen-emojiCols, 0)
			case c.Shortcut(mod, ui.KeyEnter):
				pick(true)
			default:
				return false
			}
			s.list.ScrollIntoView(s.chosen / emojiCols)
			return true
		},
		Body: func(c *ui.Context, p *Palette) {
			t := tk(c)
			s.refilter(p.query)
			rows := (len(s.filtered) + emojiCols - 1) / emojiCols
			ui.List(c, &s.list, rows, func(r int) {
				ui.Row(c).Padding(2, 8).Gap(2).Children(func() {
					for i := r * emojiCols; i < min((r+1)*emojiCols, len(s.filtered)); i++ {
						bg := ui.Transparent
						if i == s.chosen {
							bg = t.accentMuted
						}
						cell := ui.Row(c).Width(52).Height(46).Radius(10).Background(bg).Center().Children(func() {
							ui.Text(c, s.filtered[i].Glyph).FontSize(26)
						})
						if cell.Clicked() {
							s.chosen = i
						}
						if cell.DoubleClicked() {
							pick(false)
						}
					}
				})
			}).Grow(1)
		},
	}
}
