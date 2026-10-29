export type AiPricingNotice = {
  kind: 'pricing_unavailable';
  title: string;
  message: string;
};

export function pricingUnavailableNoticeFromText(
  text: string,
): AiPricingNotice | null {
  if (!/pricing[_ -]?unavailable/i.test(text)) return null;
  return {
    kind: 'pricing_unavailable',
    title: 'AI model unavailable',
    message:
      'Nevermind cannot price the selected AI model right now, so this request was not sent. Try again later or choose another model.',
  };
}
