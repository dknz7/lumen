// Lumen's icon set — every visible icon in the UI. Import from here so we
// control stroke-width consistency and can swap libraries in one place later
// if needed. Standardise on stroke-width 1.75 for mid-weight lines.
export {
  ArrowLeft,
  Home,
  Maximize2,      // enter browser fullscreen
  Minimize2,      // exit browser fullscreen
  Minus,          // zoom out
  Plus,           // zoom in (paired with slider)
  X,              // close buttons
  Search,         // search input adornment
  ChevronDown,    // expanded caret
  ChevronLeft,    // shelf pagination — scroll left arrow
  ChevronRight,   // collapsed caret + shelf pagination — scroll right arrow
  GripVertical,   // drag handle on shelves + groups
  Trash2,         // remove from Continue Watching (red hover)
  CircleCheck,    // mark Continue Watching item as watched (green hover)
  Eye,            // library visible
  EyeOff,         // library hidden
  Settings,       // left-menu settings entry
  Play,           // primary playback action
  Sparkles,       // brand logo placeholder + Recommended menu entry
  Bookmark,       // Watchlist menu entry
  Compass,        // Discover menu entry
  Library,        // Libraries section header
  Star,           // Stargaze server group + left-menu entry
  Server,         // DKNZPLEX (and any other) server group + left-menu entry
  Flame,          // Trending Movies / Trending TV shelf headers
  Film,           // Recently Released Movies shelves
  Tv,             // Recently Released Episodes shelves
  Cat,            // Recently Released Anime shelves (movies + episodes)
  ExternalLink,   // "Get a free key" link in OMDB field
  RefreshCw,      // refresh connections button
  ImageOff,       // placeholder when a thumb is missing or fails to load
} from "lucide-solid";
