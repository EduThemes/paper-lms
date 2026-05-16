package pseudonym

// Curated word lists. Reviewable in this PR. Keep entries:
//   * classroom-safe (no slurs, no body-shaming, no scary imagery)
//   * gender-neutral
//   * spelled in a single token (no spaces) so " "-split parsing works
//   * Title Case (the generator concatenates with a single space; the
//     final pseudonym reads like "Wandering Otter")
//
// Adjectives are shared across pools so the front-end picker can show
// "Curious Otter" or "Curious Sentinel" side-by-side without surprising
// the learner with a different tone in one pool.
//
// Adding to a list: pick a position that keeps it Title Case alphabetic
// (eyes-on review is easier). Removing: ok if no enrollment row has
// already chosen a pseudonym containing the removed word; otherwise
// retire the entry by deprecating in a comment and let existing rows
// keep rendering.

var sharedAdjectives = []string{
	"Bold", "Brave", "Bright", "Calm", "Cheerful", "Clever", "Curious",
	"Daring", "Dazzling", "Eager", "Friendly", "Gentle", "Glowing",
	"Happy", "Helpful", "Honest", "Joyful", "Keen", "Kind", "Lively",
	"Lucky", "Merry", "Mighty", "Nimble", "Noble", "Patient", "Peaceful",
	"Plucky", "Polite", "Proud", "Quick", "Quiet", "Radiant", "Sharp",
	"Silent", "Silly", "Sleepy", "Smart", "Sneaky", "Sparkling", "Sturdy",
	"Sunny", "Swift", "Thoughtful", "Tidy", "Wandering", "Witty",
}

var animalsPool = Pool{
	Code:        PoolAnimals,
	Label:       "Whimsical Animals",
	Description: "Friendly critters: otters, owls, foxes, and more.",
	Adjectives:  sharedAdjectives,
	Nouns: []string{
		"Otter", "Beaver", "Fox", "Lynx", "Owl", "Hawk", "Sparrow",
		"Robin", "Eagle", "Falcon", "Turtle", "Dolphin", "Whale", "Seal",
		"Walrus", "Bear", "Wolf", "Hare", "Rabbit", "Squirrel", "Badger",
		"Hedgehog", "Mouse", "Frog", "Toad", "Gecko", "Iguana", "Chameleon",
		"Tortoise", "Penguin", "Puffin", "Heron", "Crane", "Swan", "Goose",
		"Duck", "Quokka", "Wombat", "Kangaroo", "Koala", "Panda", "Tiger",
		"Lion", "Leopard", "Cheetah", "Jaguar", "Lemur", "Sloth", "Anteater",
		"Armadillo", "Bison", "Moose", "Elk", "Deer", "Antelope", "Gazelle",
		"Llama", "Alpaca", "Manatee", "Narwhal",
	},
}

var superheroesPool = Pool{
	Code:        PoolSuperheroes,
	Label:       "Brave Heroes",
	Description: "Heroic archetypes: sentinels, scholars, voyagers.",
	Adjectives:  sharedAdjectives,
	Nouns: []string{
		"Sentinel", "Beacon", "Phoenix", "Knight", "Ranger", "Scholar",
		"Guardian", "Champion", "Warden", "Pilot", "Voyager", "Mariner",
		"Inventor", "Builder", "Healer", "Tracker", "Watcher", "Sage",
		"Captain", "Pioneer", "Architect", "Cartographer", "Bard",
		"Crafter", "Smith", "Steward", "Herald", "Diplomat", "Ambassador",
		"Sentry", "Apprentice", "Marshal", "Mentor", "Vanguard", "Strider",
		"Glider", "Lookout", "Compass", "Tower", "Bridge", "Anchor",
		"Storyteller", "Mapmaker", "Stargazer", "Lantern", "Falconer",
		"Galleon", "Comet", "Nova", "Aurora", "Meridian", "Atlas",
		"Tempest", "Zephyr", "Tide", "Polaris", "Orion", "Helios",
		"Daybreak", "Twilight",
	},
}

var explorersPool = Pool{
	Code:        PoolExplorers,
	Label:       "Wandering Explorers",
	Description: "Natural-world wanderers: comets, lighthouses, glaciers.",
	Adjectives:  sharedAdjectives,
	Nouns: []string{
		"Comet", "Galaxy", "Aurora", "Meteor", "Nebula", "Mountain",
		"Glacier", "Canyon", "Geyser", "Volcano", "Tundra", "Savanna",
		"Prairie", "Reef", "Atoll", "Lagoon", "Cliff", "Cove", "Delta",
		"Estuary", "Fjord", "Forest", "Grove", "Meadow", "Marsh", "Oasis",
		"Plateau", "Ridge", "Stream", "Brook", "River", "Spring",
		"Waterfall", "Sunset", "Sunrise", "Horizon", "Trail", "Path",
		"Voyage", "Journey", "Trek", "Expedition", "Caravan", "Lantern",
		"Telescope", "Compass", "Sail", "Rudder", "Sextant", "Surveyor",
		"Wanderer", "Pilgrim", "Nomad", "Lighthouse", "Harbor",
		"Crest", "Summit", "Vista", "Boulder", "Tideline",
	},
}
