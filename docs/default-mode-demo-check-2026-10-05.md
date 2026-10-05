# Default-mode demo checks — 5 October 2026

Four live standard-mode requests were made. No Google-grounded research request was made for these checks. Durations are client-observed; cached source reuse makes timings variable.

| Request | Duration | Restaurants | Dishes with amounts | Currency coverage |
| --- | ---: | ---: | ---: | --- |
| Barcelona vegetarian lunch, EUR 25 per dish | 109 s | 3 | 10 / 10 | EUR on all 10 |
| Paris non-vegetarian lunch, EUR 25 per dish | 50 s | 2 | 2 / 3 | EUR on both priced dishes |
| Tokyo chicken dinner, JPY 3000 per dish | 107 s | 2 | 5 / 5 | Initial run lost currency because the parser did not recognise “yen”; fixed with regression coverage |
| Tokyo chicken dinner, no budget specified | 77 s | 1 | 2 / 2 | JPY on both dishes |

## Recommended demo prompts

Barcelona, strongest observed result:

> Find vegetarian lunch in Barcelona, Spain, under EUR 25 per dish. Show dishes, descriptions, prices and menu links.

Tokyo, successful dinner run:

> we are going to Tokyo and we need a meal plan for dinner non-veg chicken

Paris, partial price coverage:

> Find non-vegetarian lunch in Paris, France, under EUR 25 per dish. Show dishes, descriptions, prices and menu links.

These exact prompts were executed. Barcelona had two confirmed restaurants and one possible option. Paris had two possible options. The successful Tokyo run returned Yakitori Miyagawa Otemachi branch, White Deep-Fried Chicken JPY 1058 and Five Assorted Yakitori JPY 1436. The broader Tokyo budget run used a lunch-set article; its prices should not be presented as verified dinner availability.

## Source sampling

- [Aguaribay menu on Guidavera](https://guidavera.com/spain/barcelona/restaurants/aguaribay): manually inspected; Koftas EUR 8 for three pieces, Seasonal Salad EUR 9.50, Veggie Balls EUR 11.50 and Patatas Bravas EUR 7.50 match the returned amounts. This is a third-party menu, not a guarantee of current restaurant prices.
- [Yakitori Miyagawa Otemachi menu on Savor Japan](https://savorjapan.com/0006118178/menus): manually inspected; both returned chicken dishes and JPY amounts appear on the page. Dinner opening hours are listed separately.
- Paris prices were supported by the retrieved menu-image extraction at France Revisited. Independent browser-source inspection did not complete in this check; the restaurants remain possible options.

## Rendering and regression verification

Missing dish amounts omit the price slot in both modes. Supported per-person estimates remain labelled. Grounded prices in a separate attributed pricing paragraph are recovered within the same dish block; the recovery preserves ranges, approximate labels, currency and open-ended amounts. It must not borrow a price from the next dish or an unattributed paragraph. Existing saved cards can recover this layout without another paid request.

Seven frontend tests passed. Backend tests, PostgreSQL integration tests, vet, lint and targeted race checks passed. Browser automation replayed the live Paris and Barcelona records and a grounded multi-paragraph yen-price fixture, checking visible prices, hidden absent-price slots and JavaScript errors. Grounded rendering was tested without a paid rerun.
