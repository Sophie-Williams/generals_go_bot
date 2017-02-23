package main

import (
	"log"
	"math"
	"os"
	"time"
	"github.com/xarg/gopathfinding"
	"github.com/andyleap/gioframework"
)

const (
	TILE_EMPTY = -1
	TILE_MOUNTAIN = -2
	TILE_FOG = -3
	TILE_FOG_OBSTACLE = -4
)

func main() {

	client, _ := gioframework.Connect("bot", os.Getenv("GENERALS_BOT_ID"), "Terminator")
	go client.Run()

	num_games_to_play := 1

	for i := 0; i < num_games_to_play; i++ {
		var game *gioframework.Game
		if os.Getenv("REAL_GAME") == "true" {
			game = client.Join1v1()
		} else {
			game_id := "bot_testing_game"
			game = client.JoinCustomGame(game_id)
			url := "http://bot.generals.io/games/" + game_id
			log.Printf("Joined custom game, go to: %v", url)
			game.SetForceStart(true)
		}

		started := false
		game.Start = func(playerIndex int, users []string) {
			log.Println("Game started with ", users)
			started = true
		}
		done := false
		game.Won = func() {
			log.Println("Won game!")
			done = true
		}
		game.Lost = func() {
			log.Println("Lost game...")
			done = true
		}
		for !started {
			time.Sleep(1 * time.Second)
		}

		time.Sleep(1 * time.Second)

		for !done {
			time.Sleep(100 * time.Millisecond)
			if game.QueueLength() > 0 {
				continue
			}

			// Re-enable after debugging...
			//if game.TurnCount < 20 {
			//	log.Println("Waiting for turn 20...")
			//	continue
			//}


			from, to_target, score := GetTileToAttack(game)
			if from < 0 {
				continue
			}
			path := GetShortestPath(game, from, to_target)
			to := path[1]

			log.Printf("Moving army %v: %v -> %v (Score: %v)",
				       game.GameMap[from].Armies, game.GetCoordString(from),
			           game.GetCoordString(to), score)
			//mine := []int{}
			//for i, tile := range game.GameMap {
			//	if tile.Faction == game.PlayerIndex && tile.Armies > 1 {
			//		mine = append(mine, i)
			//	}
			//}
			//if len(mine) == 0 {
			//	continue
			//}
			//cell := rand.Intn(len(mine))
			//move := []int{}
			//for _, adjacent := range game.GetAdjacents(mine[cell]) {
			//	if game.Walkable(adjacent) {
			//		move = append(move, adjacent)
			//	}
			//}
			//if len(move) == 0 {
			//	continue
			//}
			//movecell := rand.Intn(len(move))
			game.Attack(from, to, false)

		}
	}
}

func Btoi(b bool) int {
    if b {
        return 1
    }
    return 0
}

func Btof(b bool) float64 {
    if b {
        return 1.
    }
    return 0.
}

func GetShortestPath(game *gioframework.Game, from, to int) []int {
	map_data := *pathfinding.NewMapData(game.Height, game.Width)
	for i := 0; i <  game.Height; i++ {
		for j := 0; j < game.Width; j++ {
			map_data[i][j] = Btoi(!game.Walkable(game.GetIndex(i, j)))
		}
	}
	map_data[game.GetRow(from)][game.GetCol(from)] = pathfinding.START
	map_data[game.GetRow(to)][game.GetCol(to)] = pathfinding.STOP

	graph := pathfinding.NewGraph(&map_data)
	nodes_path := pathfinding.Astar(graph)
	path := []int{}
	for _, node := range nodes_path {
		path = append(path, game.GetIndex(node.X, node.Y))
	}
	return path
}

func GetTileToAttack(game *gioframework.Game) (int, int, float64) {

	best_from := -1
	best_to := -1
	best_total_score := 0.
	var best_scores map[string]float64

	my_general := game.Generals[game.PlayerIndex]


	for from, from_tile := range game.GameMap {
		if from_tile.Faction != game.PlayerIndex || from_tile.Armies < 2 {
			continue
		}
		//my_army_size := from_tile.Armies

		for to, to_tile := range game.GameMap {
			//log.Println(from, to)
			if to_tile.Faction < -1 {
				continue
			}
			// Note: I'm not dealing with impossible to reach tiles for now
			// No gathering for now...
			if to_tile.Faction == game.PlayerIndex {
				continue
			}

			is_empty := to_tile.Faction == TILE_EMPTY
			is_enemy := to_tile.Faction != game.PlayerIndex && to_tile.Faction >= 0
			is_general := to_tile.Type == gioframework.General
			is_city := to_tile.Type == gioframework.City
			outnumber := float64(from_tile.Armies - to_tile.Armies)
			// Should I translate my heuristic distance from my JS code?
			dist := float64(game.GetDistance(from, to))
			dist_from_gen := float64(game.GetDistance(my_general, to))

			scores := make(map[string]float64)

			scores["outnumber_score"] = Truncate(outnumber / 300, 0., 0.2)
			scores["outnumbered_penalty"] = -0.1 * Btof(outnumber < 2)
			log.Printf("dist_from_gen: %v, %v", dist_from_gen, Btof(is_enemy))
			scores["general_threat_score"] = (0.2 * math.Pow(dist_from_gen, -0.1)) * Btof(is_enemy)
			scores["dist_penalty"] = Truncate(-0.2 * dist / 30, -0.2, 0)
			scores["dist_gt_army_penalty"] = -0.1 * Btof(from_tile.Armies < int(dist))
			scores["is_enemy_score"] = 0.1 * Btof(is_enemy)
			scores["close_city_score"] = 0.1 * Btof(is_city) * math.Pow(dist_from_gen, -0.5)
			scores["enemy_gen_score"] = 0.1 * Btof(is_general) * Btof(is_enemy)
			scores["empty_score"] = 0.05 * Btof(is_empty)

			total_score := 0.
			for k, score := range scores {
				log.Printf("%v, %v", k, score)
				total_score += score
			}
			log.Printf("total score is: %v", total_score)

			if total_score > best_total_score {
				best_scores = scores
				best_total_score = total_score
				best_from = from
				best_to = to
			}

		}
	}
	log.Println("Good")
	for name, score := range best_scores {
		log.Printf("%v: %v\n", name, score)
	}
	return best_from, best_to, best_total_score
}


func Truncate(val, min, max float64) float64 {
    return math.Min(math.Max(val, min), max)
}