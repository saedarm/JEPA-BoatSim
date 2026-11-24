# What-If Boat

A tiny game about telling a bad decision from bad luck.

You sail a boat through water with a current you can't see. After the trip, you pick a different heading and ask: *what would have happened if I'd gone that way instead, in the same current?* The game shows two guesses, then replays the trip with the same current so you can see which guess was closer.

It's a playable version of the trick from the paper [Counterfactual World Models: JEPAs That Reason About "What If"](https://arxiv.org/abs/2611.24778) by Randall Balestriero, Yann LeCun and Kerda Varaku (arXiv preprint, Nov 2026).

## How Do I Run It?

You need [Go](https://go.dev/dl/) 1.25 or newer.

```
go run .
```

The first run downloads [Ebiten](https://ebitengine.org/), the 2D game library this uses. To build a standalone executable instead:

```
go build -o whatif-boat.exe .
```

## How Do I Play?

1. **Aim and sail.** Move the mouse to aim from the starting dot, then click. The grey ring is where the model thinks you'll land. The current pushes you somewhere else.
2. **Pick your what-if.** Aim a different heading. Two markers follow your mouse:
   - **Grey ring:** the plain model's guess for that heading. It knows how currents behave on average, but not which one you hit.
   - **Blue dot:** the what-if answer. It takes where you actually landed and shifts it by the model's predicted difference between the two headings.
3. **Reveal.** Click, and a green ghost boat sails your new heading through the *same* current. The top bar tells you how far off each guess was.

The bottom of the screen keeps a running average of both misses across trips.

### Controls

| Key | What it does |
|---|---|
| Mouse | Aim |
| Left click | Sail / replay / next trip |
| Up / Down | Make trips longer or shorter (while aiming) |
| C | Show or hide the current |
| R | Start a new trip |

## What's the Trick?

One line:

```go
answer := landed.Add(guessInstead.Sub(guessDid))
```

Where you actually landed already has this trip's current baked into it. So instead of trying to figure out the current, you keep the real outcome and only ask the model how much the *decision* changes things. Two guesses and one addition, no extra training.

## When Does It Break?

Try making trips longer with the Up key. The trick works because over a short trip, the engine's push and the current's push mostly add up on their own. On long trips, a different heading carries you into different water, the two pushes start tangling together, and the blue dot drifts away from the truth. Short trips: nearly perfect. Long trips: not so much.

The plain model will occasionally win a round. That's luck too.

## What's in Here?

- `main.go` — the whole game: the physics, the "world model," and the drawing
- `go.mod` / `go.sum` — dependencies (just Ebiten)
