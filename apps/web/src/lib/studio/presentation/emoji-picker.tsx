import { useState } from 'react';
import { emojiChoices, type Translator } from '@hourpaths/i18n';
export function EmojiPicker({ i18n, choose, disabled = false }: { i18n: Translator; choose(emoji: string): void; disabled?: boolean }) {
  const [search, setSearch] = useState('');
  const [limit, setLimit] = useState(60);
  const choices = emojiChoices.filter(item => i18n.t(item.key).toLocaleLowerCase(i18n.locale).includes(search.toLocaleLowerCase(i18n.locale)) || item.emoji.includes(search));
  return <div className="studio-picker"><label>{i18n.t('emojiPicker.search')}<input type="search" value={search} onChange={event => { setSearch(event.target.value); setLimit(60); }} /></label>
    <div className="studio-emoji-choices">{choices.slice(0, limit).map(choice => <button type="button" key={choice.key} title={i18n.t(choice.key)} aria-label={i18n.t(choice.key)} disabled={disabled} onClick={() => choose(choice.emoji)}>{choice.emoji}</button>)}</div>
    {!choices.length && <p>{i18n.t('emojiPicker.empty')}</p>}
    {choices.length > limit && <button type="button" onClick={() => setLimit(limit + 60)}>{i18n.t('studio.moreEmoji')}</button>}
  </div>;
}
