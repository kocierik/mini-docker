import Reveal from 'reveal.js';
import RevealHighlight from 'reveal.js/plugin/highlight';
import RevealNotes from 'reveal.js/plugin/notes';
import 'reveal.js/reveal.css';
import 'reveal.js/plugin/highlight/monokai.css';
import '@fontsource/ibm-plex-sans-condensed/400.css';
import '@fontsource/ibm-plex-sans-condensed/600.css';
import '@fontsource/ibm-plex-sans-condensed/700.css';
import '@fontsource/ibm-plex-mono/400.css';
import '@fontsource/ibm-plex-mono/600.css';
import './theme.css';

const deck = new Reveal(document.querySelector('.reveal'), {
  width: 1600,
  height: 900,
  margin: 0.04,
  hash: true,
  controls: true,
  progress: true,
  center: false,
  transition: 'none',
  backgroundTransition: 'none',
  plugins: [RevealHighlight, RevealNotes],
});

window.presentationDeck = deck;
await deck.initialize();